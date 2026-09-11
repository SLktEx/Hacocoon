//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"errors"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWSLOperationsUseGUIDAndFixedCommands(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	r := registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}
	stop, err := r.wslArguments(wslStop)
	if err != nil {
		t.Fatal(err)
	}
	prefix := []string{"--distribution-id", id.String(), "--user", "root", "--cd", "/", "--exec", "/usr/bin/env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin"}
	if !reflect.DeepEqual(stop, append(prefix, "/usr/bin/systemctl", "--no-block", "poweroff")) {
		t.Fatal(stop)
	}
	resume, err := r.wslArguments(wslResume)
	if err != nil || !reflect.DeepEqual(resume, append(prefix, "/usr/bin/true")) {
		t.Fatal(resume, err)
	}
	read, err := r.wslArguments(wslReadRegistration)
	if err != nil || !reflect.DeepEqual(read, append(prefix, "/usr/bin/python3", "-I", "/usr/local/libexec/hacocoon-wsl-interop", "--read-registration")) {
		t.Fatal("registration read must use the fixed isolated helper", read, err)
	}
	r.Name = "Same-GUID-Renamed"
	if renamed, err := r.wslArguments(wslStop); err != nil || !reflect.DeepEqual(renamed, stop) {
		t.Fatal("stop depends on name", renamed, err)
	}
	if _, err := r.wslArguments(0); err == nil {
		t.Fatal("unknown operation accepted")
	}
	r.ID = windows.GUID{}
	if _, err := r.wslArguments(wslStop); err == nil {
		t.Fatal("default distribution accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := r.reclaimWithResume(ctx)
	if !errors.Is(err, context.Canceled) || result.StopAttempted || result.ResumeAttempted {
		t.Fatal(result, err)
	}
}

func TestDedicatedWSLContinuation(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_CONTINUATION") != "1" {
		t.Skip("requires separate exact managed WSL stop/compact/resume authorization")
	}
	r, err := readRegistration(os.Getenv("HACO_E2E_RECLAIM_REGISTRATION"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := r.diskPath()
	if err != nil || path != os.Getenv("HACO_E2E_RECLAIM_VHD") {
		t.Fatal("registered VHD path mismatch", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	result, err := r.reclaimWithResume(ctx)
	if err != nil {
		t.Fatalf("continuation failed; observation=%+v error=%v", result, err)
	}
	if !result.StopRequested || !result.Compaction.Completed || !result.Resumed {
		t.Fatal(result)
	}
	t.Logf("PASS native continuation: %+v; controller/data acceptance checked separately", result)
}

func TestContinuationPreservesFailureAndAlwaysAttemptsBoundedResume(t *testing.T) {
	stopFailed := errors.New("stop failed")
	compactFailed := errors.New("compact failed")
	resumeFailed := errors.New("resume failed")
	for _, tc := range []struct {
		name                           string
		stopErr, compactErr, resumeErr error
		cancel                         bool
	}{
		{"success", nil, nil, nil, false},
		{"stop", stopFailed, nil, nil, false},
		{"compact", nil, compactFailed, nil, false},
		{"resume", nil, nil, resumeFailed, false},
		{"both", nil, compactFailed, resumeFailed, false},
		{"cancel-after-stop", nil, context.Canceled, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var order []string
			result, err := executeContinuation(ctx,
				func(context.Context) error {
					order = append(order, "stop")
					if tc.cancel {
						cancel()
					}
					return tc.stopErr
				},
				func(context.Context) (compactObservation, error) {
					order = append(order, "compact")
					return compactObservation{Attempted: true, Completed: tc.compactErr == nil}, tc.compactErr
				},
				func(resumeCtx context.Context) error {
					order = append(order, "resume")
					deadline, ok := resumeCtx.Deadline()
					if !ok || resumeCtx.Err() != nil || time.Until(deadline) > 2*time.Minute {
						t.Fatal("resume context is not independent and bounded")
					}
					return tc.resumeErr
				})
			expected := []string{"stop", "compact", "resume"}
			if tc.stopErr != nil {
				expected = []string{"stop", "resume"}
			}
			if !reflect.DeepEqual(order, expected) || !result.StopAttempted || !result.ResumeAttempted || result.StopRequested != (tc.stopErr == nil) || result.Resumed != (tc.resumeErr == nil) {
				t.Fatal(order, result, err)
			}
			for _, cause := range []error{tc.stopErr, tc.compactErr, tc.resumeErr} {
				if cause != nil && !errors.Is(err, cause) {
					t.Fatal("lost failure", cause, err)
				}
			}
			if tc.stopErr == nil && tc.compactErr == nil && tc.resumeErr == nil && err != nil {
				t.Fatal(err)
			}
			if tc.stopErr != nil && result.Compaction.Attempted {
				t.Fatal("compaction after stop failure")
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := executeContinuation(ctx, nil, nil, nil)
	if !errors.Is(err, context.Canceled) || result.StopAttempted || result.ResumeAttempted {
		t.Fatal(result, err)
	}
}

func TestPreparedContinuationRejectsBeforeAccess(t *testing.T) {
	r := registration{}
	if result, err := r.continuePrepared(context.Background(), windows.GUID{}); err == nil || result.StopAttempted {
		t.Fatal(result, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if intent, err := r.prepareContinuation(ctx); !errors.Is(err, context.Canceled) || intent != (operationRecord{}) {
		t.Fatal(intent, err)
	}
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	if result, err := r.continuePrepared(ctx, id); !errors.Is(err, context.Canceled) || result.StopAttempted {
		t.Fatal(result, err)
	}
}

func TestDedicatedWSLPreparedContinuation(t *testing.T) {
	if os.Getenv("HACO_E2E_RECLAIM_HANDOFF") != "1" {
		t.Skip("requires exact dedicated prepared stop/compact/resume authorization")
	}
	r, err := readRegistration(os.Getenv("HACO_E2E_RECLAIM_REGISTRATION"))
	if err != nil {
		t.Fatal(err)
	}
	path, err := r.diskPath()
	if err != nil || path != os.Getenv("HACO_E2E_RECLAIM_VHD") {
		t.Fatal("registered VHD path mismatch", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	prepared, err := PrepareWorker(ctx, r.ID.String())
	if err != nil {
		t.Fatal("prepare failed; preserve any saved intent", err)
	}
	if os.Getenv("HACO_E2E_RECLAIM_PREPARE_ONLY") == "1" {
		t.Logf("PREPARED_OPERATION=%s; pending only, execution is not complete", prepared.Operation)
		return
	}
	// Preparation has released its guard and file handles. Reopen the persisted
	// record before executing, as a worker in another process must do.
	records, err := openOperationStore(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer records.close()
	saved, err := records.read()
	if err != nil || saved.Operation.String() != prepared.Operation || saved.State != prepared.State || saved.State != "pending" {
		t.Fatal("prepared identity not durable", saved, err)
	}
	intent := saved
	result, err := r.continuePrepared(ctx, intent.Operation)
	if err != nil {
		t.Fatalf("prepared continuation failed; observation=%+v error=%v", result, err)
	}
	completed, err := records.read()
	if err != nil || completed.Operation != intent.Operation || completed.Registration != intent.Registration || completed.Disk != intent.Disk || completed.State != "complete" || completed.Observation != result {
		t.Fatal("handoff result differs from exact prepared identity", completed, err)
	}
	t.Logf("PASS prepared native continuation: %+v; separate-process/public/controller/data acceptance is separate", result)
}

func TestContinuationFailureDiagnosticsAreFixedAndPreservePrimaryFailure(t *testing.T) {
	for _, tc := range []struct {
		stage                 string
		stop, compact, resume error
		code                  uint32
	}{
		{stage: "stop", stop: syscall.Errno(5), code: 5},
		{stage: "compact", compact: syscall.Errno(32), code: 32},
		{stage: "compact_attached", compact: errVirtualDiskAttached, resume: syscall.Errno(5)},
		{stage: "resume", resume: syscall.Errno(5), code: 5},
	} {
		result, err := executeContinuation(context.Background(), func(context.Context) error { return tc.stop }, func(context.Context) (compactObservation, error) { return compactObservation{}, tc.compact }, func(context.Context) error { return tc.resume })
		if err == nil || result.Failure != tc.stage || result.NativeError != tc.code {
			t.Fatal(result, err)
		}
	}
}
