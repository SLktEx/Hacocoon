package wslreclaim

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

func TestObservedStartPreservesSingleCallAndPrimaryError(t *testing.T) {
	primary := &os.PathError{Op: "fork/exec", Path: "private-path", Err: syscall.Errno(5)}
	for _, fail := range []bool{false, true} {
		var sequence []string
		observed := 0
		err := startObservedWorker(context.Background(), func() error {
			sequence = append(sequence, "start")
			if fail {
				return primary
			}
			return nil
		}, func() reclamation.JobObservation {
			observed++
			sequence = append(sequence, "observe")
			// Failing diagnostics must not replace the captured Start error or code.
			return unavailableJob("limits", syscall.Errno(32))
		})
		want := []string{"observe", "start"}
		if fail {
			want = append(want, "observe")
		}
		if !reflect.DeepEqual(sequence, want) {
			t.Fatal(sequence)
		}
		if !fail {
			if err != nil || observed != 1 {
				t.Fatal(err, observed)
			}
			continue
		}
		if !errors.Is(err, primary) || !errors.Is(err, syscall.Errno(5)) || errors.Is(err, syscall.Errno(32)) {
			t.Fatal(err)
		}
		if err.Error() != "Windows worker process_start failed: "+primary.Error() {
			t.Fatal("primary error changed", err)
		}
		f := WorkerLaunchFailure(err)
		if !f.Valid() || f.NativeError != 5 || f.Stage != "process_start" || f.Job == nil || f.Job.OuterJobs != "unknown" || f.Job.AfterFailure.NativeError != 32 {
			t.Fatal(f)
		}
		raw, _ := json.Marshal(f)
		for _, private := range []string{"private-path", "fork/exec", "pid", "command", "environment"} {
			if strings.Contains(string(raw), private) {
				t.Fatal("private diagnostic", string(raw))
			}
		}
	}
}

func TestObservedStartPreservesCancellation(t *testing.T) {
	for _, before := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		if before {
			cancel()
		}
		observations := 0
		err := startObservedWorker(ctx, func() error { t.Fatal("canceled launch started"); return nil }, func() reclamation.JobObservation {
			observations++
			cancel()
			return reclamation.JobObservation{State: "not_in_job"}
		})
		cancel()
		if !errors.Is(err, context.Canceled) || (before && observations != 0) || (!before && observations != 1) {
			t.Fatal(err, observations)
		}
	}
}

func TestObserveJobDistinguishesUnavailableFromNoJob(t *testing.T) {
	for _, tc := range []struct {
		name                string
		in                  bool
		memberErr, limitErr error
		flags               uint32
		state, at           string
		code                uint32
		limitCalls          int
		wantFlags           [3]bool
	}{
		{name: "no job", state: "not_in_job"},
		{name: "membership denied", memberErr: syscall.Errno(5), state: "unavailable", at: "membership", code: 5},
		{name: "unknown membership", memberErr: errors.New("token=secret"), state: "unavailable", at: "membership"},
		{name: "limits denied", in: true, limitErr: syscall.Errno(32), state: "unavailable", at: "limits", code: 32, limitCalls: 1},
		{name: "zero flags observed", in: true, state: "observed", limitCalls: 1},
		{name: "only allowlisted flags", in: true, flags: 0xffffffff, state: "observed", limitCalls: 1, wantFlags: [3]bool{true, true, true}},
		{name: "breakaway", in: true, flags: 0x800, state: "observed", limitCalls: 1, wantFlags: [3]bool{true, false, false}},
		{name: "silent breakaway", in: true, flags: 0x1000, state: "observed", limitCalls: 1, wantFlags: [3]bool{false, true, false}},
		{name: "kill on close", in: true, flags: 0x2000, state: "observed", limitCalls: 1, wantFlags: [3]bool{false, false, true}},
		{name: "unrelated limit", in: true, flags: 0x400, state: "observed", limitCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			got := observeJob(func() (bool, error) { return tc.in, tc.memberErr }, func() (uint32, error) { calls++; return tc.flags, tc.limitErr })
			if !got.Valid() || got.State != tc.state || got.UnavailableAt != tc.at || got.NativeError != tc.code || calls != tc.limitCalls {
				t.Fatal(got, calls)
			}
			if got.State == "observed" {
				if [3]bool{*got.Flags.BreakawayOK, *got.Flags.SilentBreakawayOK, *got.Flags.KillOnJobClose} != tc.wantFlags {
					t.Fatal(got.Flags)
				}
			} else if got.Flags != nil {
				t.Fatal("unknown converted to false flags")
			}
		})
	}
}

// This cross-platform source regression protects the actual Windows call site
// as well as the injectable sequencing test; it does not claim native acceptance.
func TestWindowsWorkerLaunchWiringUnchanged(t *testing.T) {
	raw, err := os.ReadFile("worker_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, want := range []string{
		`exec.Command(executable, "_continue", r.String(), o.String())`,
		`child.Dir = system`, `child.Env = []string{"SystemRoot=" + root, "WINDIR=" + root}`,
		`HideWindow: true, CreationFlags: 0x00000008 | 0x01000000 | 0x00000200`,
		`startObservedWorker(ctx, child.Start, observeLauncherJob)`,
		`child.Stdout = write`, `readyErr := readWorkerReady(ctx, read)`,
	} {
		if !strings.Contains(source, want) {
			t.Fatal("launch contract changed", want)
		}
	}
	if strings.Count(source, "child.Start") != 1 || strings.Contains(source, "CommandContext(") {
		t.Fatal("launch duplicated or tied to caller cancellation")
	}
}
