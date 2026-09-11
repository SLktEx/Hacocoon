//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/reclamation"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestPreparedWorkerRejectsInvalidRequestBeforeLaunch(t *testing.T) {
	id := "{11111111-1111-4111-8111-111111111111}"
	for _, args := range [][2]string{{"", id}, {id, ""}, {"--shutdown", id}, {id, "{00000000-0000-0000-0000-000000000000}"}} {
		if pid, err := LaunchPreparedWorker(context.Background(), args[0], args[1]); err == nil || pid != 0 {
			t.Fatal(pid, err)
		}
		if err := ExecutePreparedWorker(context.Background(), args[0], args[1]); err == nil {
			t.Fatal("invalid worker request accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if pid, err := LaunchPreparedWorker(ctx, id, id); !errors.Is(err, context.Canceled) || pid != 0 {
		t.Fatal(pid, err)
	}
	if err := ExecutePreparedWorker(ctx, id, id); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestPinnedExecutableExcludesWriteAndRename(t *testing.T) {
	var empty pinnedDisk
	if err := empty.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := empty.Allocation(); err == nil {
		t.Fatal("empty disk has allocation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "worker.exe")
	if err := os.WriteFile(path, []byte("isolated fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	pin, err := pinLocalFile(path, ".exe", false)
	if err != nil {
		t.Fatal(err)
	}
	defer pin.Close()
	if file, err := os.OpenFile(path, os.O_WRONLY, 0); err == nil {
		file.Close()
		t.Fatal("pinned executable writable")
	}
	if err := os.Rename(path, path+".moved"); err == nil {
		t.Fatal("pinned executable renamed")
	}
	if _, err := pinDisk(path); err == nil {
		t.Fatal("executable accepted as disk")
	}
	if err := pin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatal("executable pin leaked", err)
	}
}

func TestPreparedStatusIsReadOnlyAndPendingIsUnknown(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	operation, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	path := `Software\Hacocoon\Reclamation\` + id.String()
	if _, err := ReadPreparedStatus(context.Background(), id.String(), operation.String()); err == nil {
		t.Fatal("missing result accepted")
	}
	if _, err := ReadLatestPreparedStatus(context.Background(), id.String()); err == nil {
		t.Fatal("missing latest result accepted")
	}
	if key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE); err == nil {
		key.Close()
		t.Fatal("status created a key")
	}
	key, existed, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	if existed {
		key.Close()
		t.Fatal("fixture collision")
	}
	defer func() {
		key.Close()
		if err := registry.DeleteKey(registry.CURRENT_USER, path); err != nil {
			t.Error(err)
		}
	}()
	record := operationRecord{Version: 1, Operation: operation, Registration: registration{ID: id, Name: "Hacocoon-Test", BasePath: `C:\owned`, VHDFileName: "ext4.vhdx"}, Disk: diskIdentity{Volume: 1, Low: 2}, State: "pending"}
	store := &operationStore{key: key}
	if err := store.write(record); err != nil {
		t.Fatal(err)
	}
	before, _, err := key.GetBinaryValue("Operation")
	if err != nil {
		t.Fatal(err)
	}
	status, err := ReadPreparedStatus(context.Background(), id.String(), operation.String())
	if err != nil || status.State != "pending" || status.Observation != nil {
		t.Fatal("pending claimed native results", status, err)
	}
	latest, err := ReadLatestPreparedStatus(context.Background(), id.String())
	if err != nil || latest != status {
		t.Fatal("latest pending result changed identity or claimed completion", latest, err)
	}
	raw, err := json.Marshal(status)
	if err != nil || bytes.Contains(raw, []byte("observation")) {
		t.Fatal(string(raw), err)
	}
	if _, err := ReadPreparedStatus(context.Background(), id.String(), id.String()); err == nil {
		t.Fatal("foreign operation accepted")
	}
	after, _, err := key.GetBinaryValue("Operation")
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("read changed saved bytes", err)
	}
	if err := store.finish(record, continuationObservation{StopAttempted: true}, errors.New("failed")); err != nil {
		t.Fatal(err)
	}
	status, err = ReadPreparedStatus(context.Background(), id.String(), operation.String())
	if err != nil || status.State != "failed" || status.Observation == nil || !status.Observation.StopAttempted || status.Observation.Resumed {
		t.Fatal("failed result changed", status, err)
	}
	latest, err = ReadLatestPreparedStatus(context.Background(), id.String())
	if err != nil || latest.Operation != status.Operation || latest.State != "failed" || latest.Observation == nil || *latest.Observation != *status.Observation {
		t.Fatal("latest failed result changed", latest, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadLatestPreparedStatus(ctx, id.String()); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled latest inspection proceeded", err)
	}
	for _, invalid := range []string{"", "--shutdown", windows.GUID{}.String()} {
		if _, err := ReadLatestPreparedStatus(context.Background(), invalid); err == nil {
			t.Fatal("invalid latest registration accepted")
		}
	}
	// The new report is available through the same read-only status route,
	// including an attempted operation whose result was lost.
	v2 := record
	v2.Version, v2.State, v2.LinuxStarted = 2, "failed", true
	failed := reclamation.NotStarted("identity_changed")
	v2.Linux = &failed
	if err := store.write(v2); err != nil {
		t.Fatal(err)
	}
	status, err = ReadLatestPreparedStatus(context.Background(), id.String())
	if err != nil || !status.LinuxStarted || status.Linux == nil || status.Linux.Failure != "identity_changed" {
		t.Fatal("Linux evidence missing from status", status, err)
	}
	v2.Linux = nil
	if err := store.write(v2); err != nil {
		t.Fatal(err)
	}
	status, err = ReadLatestPreparedStatus(context.Background(), id.String())
	if err != nil || !status.LinuxStarted || status.Linux != nil || status.State != "failed" {
		t.Fatal("lost Linux result claimed success", status, err)
	}

	v2.State = "pending"
	if err := store.write(v2); err != nil {
		t.Fatal(err)
	}
	if err := store.reviewInterrupted(v2.Operation, v2.Registration, v2.Disk); err != nil {
		t.Fatal(err)
	}
	status, err = ReadLatestPreparedStatus(context.Background(), id.String())
	if err != nil || status.State != "interrupted" || status.Observation != nil || !status.LinuxStarted || status.Linux != nil {
		t.Fatal("interrupted status invented results", status, err)
	}
	if err := store.key.DeleteValue(interruptedReviewName(v2.Operation)); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLatestPreparedStatus(context.Background(), id.String()); err == nil {
		t.Fatal("status claimed missing evidence was retained")
	}
	// A current record is not permission to follow another registration or to
	// ignore malformed state in favor of historical evidence.
	foreign := record
	foreign.Registration.ID = operation
	if err := store.write(foreign); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLatestPreparedStatus(context.Background(), id.String()); err == nil {
		t.Fatal("foreign current result accepted")
	}
	broken := []byte("not a canonical operation")
	if err := key.SetBinaryValue("Operation", broken); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadLatestPreparedStatus(context.Background(), id.String()); err == nil {
		t.Fatal("malformed latest result accepted")
	}
	after, _, err = key.GetBinaryValue("Operation")
	if err != nil || !bytes.Equal(after, broken) {
		t.Fatal("latest read modified malformed state", err)
	}
}

func TestWorkerConsoleBoundary(t *testing.T) {
	console, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow").Call()
	err := requireIndependentWorker()
	if (console != 0) != (err != nil) {
		t.Fatalf("console boundary mismatch: console=%v error=%v", console != 0, err)
	}
}

func TestWorkerReadinessRequiresExactFrameAndEOF(t *testing.T) {
	for _, frame := range []string{"RDY\n", "", "RDY", "RDY\nextra", "FAIL"} {
		read, write, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := write.Write([]byte(frame)); err != nil {
			t.Fatal(err)
		}
		write.Close()
		err = readWorkerReady(context.Background(), read)
		read.Close()
		if (err == nil) != (frame == "RDY\n") {
			t.Fatalf("readiness frame rejected/accepted incorrectly: %v", err)
		}
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer write.Close()
	defer read.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := readWorkerReady(ctx, read); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

// Production preparation refuses before touching an enrollment/operation when
// the registration is malformed, absent or the caller is canceled.
func TestPrepareWorkerRefusesInvalidOrAbsentRegistration(t *testing.T) {
	id, err := windows.GenerateGUID()
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"", "--shutdown", windows.GUID{}.String(), id.String()} {
		got, err := PrepareWorker(context.Background(), input)
		if err == nil || got != (PreparedStatus{}) {
			t.Fatal("invalid preparation accepted", got, err)
		}
	}
	path := `Software\Hacocoon\Reclamation\` + id.String()
	if key, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE); err == nil {
		key.Close()
		t.Fatal("missing registration created an operation key")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := PrepareWorker(ctx, id.String()); !errors.Is(err, context.Canceled) || got != (PreparedStatus{}) {
		t.Fatal("canceled preparation proceeded", got, err)
	}
}

func TestWorkerLaunchDiagnosticsUseFixedStagesAndNativeCodes(t *testing.T) {
	for _, tc := range []struct {
		err   error
		stage string
		code  uint32
	}{
		{&workerLaunchError{stage: "process_start", err: syscall.Errno(5)}, "process_start", 5},
		{errors.Join(&workerLaunchError{stage: "pin_executable", err: syscall.Errno(32)}, errors.New("close")), "pin_executable", 32},
		{&workerLaunchError{stage: "readiness", err: errors.New("token=private")}, "readiness", 0},
		{&workerLaunchError{stage: "token=private", err: errors.New("private")}, "other", 0},
	} {
		stage, code := WorkerLaunchFailure(tc.err)
		if stage != tc.stage || code != tc.code {
			t.Fatal(stage, code)
		}
	}
}
