//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func preparedIDs(registrationID, operationID string) (windows.GUID, windows.GUID, error) {
	r, err := windows.GUIDFromString(registrationID)
	if err != nil || r == (windows.GUID{}) {
		return r, windows.GUID{}, errors.New("exact registration GUID required")
	}
	o, err := windows.GUIDFromString(operationID)
	if err != nil || o == (windows.GUID{}) {
		return r, o, errors.New("exact prepared operation GUID required")
	}
	return r, o, nil
}

// PrepareWorker durably records a new operation for the enrolled registration.
// It does not launch a worker or stop WSL. An error after persistence may leave
// pending evidence; inspect it rather than retrying or clearing it automatically.
func PrepareWorker(ctx context.Context, registrationID string) (PreparedStatus, error) {
	if err := ctx.Err(); err != nil {
		return PreparedStatus{}, err
	}
	r, err := readRegistration(registrationID)
	if err != nil {
		return PreparedStatus{}, err
	}
	intent, err := r.prepareContinuation(ctx)
	if err != nil {
		return PreparedStatus{}, err
	}
	return PreparedStatus{Operation: intent.Operation.String(), State: intent.State}, nil
}

// LaunchPreparedWorker launches this helper's fixed worker mode, not a caller
// executable or command. A PID reports dispatch only; the durable record owns
// completion. No retry or fallback to a child tied to the caller's Job is allowed.
func LaunchPreparedWorker(ctx context.Context, registrationID, operationID string) (pid int, err error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r, o, err := preparedIDs(registrationID, operationID)
	if err != nil {
		return 0, err
	}
	executable, err := os.Executable()
	if err != nil {
		return 0, err
	}
	pin, err := pinLocalFile(executable, ".exe", false)
	if err != nil {
		return 0, err
	}
	defer func() { err = errors.Join(err, pin.Close()) }()
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return 0, err
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		return 0, err
	}
	child := exec.Command(executable, "_continue", r.String(), o.String())
	child.Dir = system
	child.Env = []string{"SystemRoot=" + root, "WINDIR=" + root}
	// DETACHED_PROCESS | CREATE_BREAKAWAY_FROM_JOB | CREATE_NEW_PROCESS_GROUP.
	// Stdin/stderr remain NUL. A private native stdout pipe carries readiness
	// only and is closed by the worker before any WSL stop; no caller WSL pipes.
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000008 | 0x01000000 | 0x00000200}
	read, write, err := os.Pipe()
	if err != nil {
		return 0, err
	}
	defer read.Close()
	defer write.Close()
	child.Stdout = write
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := child.Start(); err != nil {
		return 0, fmt.Errorf("launch independent Windows worker: %w", err)
	}
	pid = child.Process.Pid
	// The launcher must release its write copy to observe worker EOF.
	if closeErr := write.Close(); closeErr != nil {
		return pid, errors.Join(closeErr, child.Process.Release())
	}
	readyErr := readWorkerReady(ctx, read)
	releaseErr := child.Process.Release()
	if readyErr != nil {
		return pid, errors.Join(fmt.Errorf("worker %d did not report readiness: %w", pid, readyErr), releaseErr)
	}
	if err := releaseErr; err != nil {
		return pid, fmt.Errorf("worker dispatched but process handle release failed: %w", err)
	}
	return pid, nil
}

// requireIndependentWorker refuses an attached console before any WSL access.
// Launch still requires explicit Job breakaway. A remaining outer Job may end
// this worker; durable pending evidence must survive rather than imply success.
// Saved enrollment and the exact operation remain the mutation authority.
func requireIndependentWorker() error {
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	console, _, _ := kernel.NewProc("GetConsoleWindow").Call()
	if console != 0 {
		return errors.New("worker is bound to a Windows console")
	}
	return nil
}

func ExecutePreparedWorker(ctx context.Context, registrationID, operationID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	rID, operation, err := preparedIDs(registrationID, operationID)
	if err != nil {
		return err
	}
	if err := requireIndependentWorker(); err != nil {
		return err
	}
	r, err := readRegistration(rID.String())
	if err != nil {
		return err
	}
	_, err = r.continuePreparedReady(ctx, operation, publishWorkerReady)
	return err
}

// PreparedStatus is a read-only projection of the last persisted result. Pending
// means the outcome is unknown, not that no native operation was attempted.
type PreparedStatus struct {
	Operation   string                   `json:"operation"`
	State       string                   `json:"state"`
	Observation *continuationObservation `json:"observation,omitempty"`
}

func ReadPreparedStatus(ctx context.Context, registrationID, operationID string) (PreparedStatus, error) {
	if err := ctx.Err(); err != nil {
		return PreparedStatus{}, err
	}
	r, o, err := preparedIDs(registrationID, operationID)
	if err != nil {
		return PreparedStatus{}, err
	}
	return readPreparedStatus(ctx, r, o)
}

// ReadLatestPreparedStatus reads the current persisted result without requiring
// the caller to retain its operation ID across WSL shutdown. It never executes,
// acknowledges or replays the selected record.
func ReadLatestPreparedStatus(ctx context.Context, registrationID string) (PreparedStatus, error) {
	if err := ctx.Err(); err != nil {
		return PreparedStatus{}, err
	}
	r, err := windows.GUIDFromString(registrationID)
	if err != nil || r == (windows.GUID{}) {
		return PreparedStatus{}, errors.New("exact registration GUID required")
	}
	return readPreparedStatus(ctx, r, windows.GUID{})
}

func readPreparedStatus(ctx context.Context, r, o windows.GUID) (PreparedStatus, error) {
	if err := ctx.Err(); err != nil {
		return PreparedStatus{}, err
	}
	// Do not create a key, observe/start WSL, take over a live worker, or require a
	// live registration merely to inspect a retained interrupted operation.
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Hacocoon\Reclamation\`+r.String(), registry.QUERY_VALUE)
	if err != nil {
		return PreparedStatus{}, err
	}
	defer key.Close()
	store := &operationStore{key: key}
	var record operationRecord
	if o == (windows.GUID{}) {
		record, err = store.read()
	} else {
		record, err = store.readOperation(o)
	}
	if err != nil {
		return PreparedStatus{}, err
	}
	if (o != (windows.GUID{}) && record.Operation != o) || record.Registration.ID != r {
		return PreparedStatus{}, errors.New("saved result belongs to another operation or registration")
	}
	status := PreparedStatus{Operation: record.Operation.String(), State: record.State}
	if record.State != "pending" {
		status.Observation = &record.Observation
	}
	return status, nil
}

// Four fixed bytes plus EOF are the entire startup protocol. No guest output,
// commands or credentials pass through it. Unknown/truncated output fails closed.
func readWorkerReady(ctx context.Context, pipe io.ReadCloser) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	stop := context.AfterFunc(ctx, func() { _ = pipe.Close() })
	defer stop()
	raw, err := io.ReadAll(io.LimitReader(pipe, 5))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, []byte("RDY\n")) {
		return errors.New("invalid or missing worker readiness")
	}
	return nil
}

func publishWorkerReady() error {
	kind, err := windows.GetFileType(windows.Handle(os.Stdout.Fd()))
	if err != nil {
		return err
	}
	if kind != windows.FILE_TYPE_PIPE {
		return errors.New("worker readiness requires its private pipe")
	}
	if _, err := io.WriteString(os.Stdout, "RDY\n"); err != nil {
		return err
	}
	// The native channel must be gone before shutdown. The persisted operation,
	// rather than this pipe or launcher lifetime, carries the final result.
	return os.Stdout.Close()
}
