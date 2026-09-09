//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

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
	// Stdio defaults to NUL; no WSL pipes, console, caller environment or handles.
	child.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000008 | 0x01000000 | 0x00000200}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if err := child.Start(); err != nil {
		return 0, fmt.Errorf("launch independent Windows worker: %w", err)
	}
	pid = child.Process.Pid
	if err := child.Process.Release(); err != nil {
		return pid, fmt.Errorf("worker dispatched but process handle release failed: %w", err)
	}
	return pid, nil
}

// requireIndependentWorker refuses a console or Job-bound process before any WSL
// observation or stop. The process is not an authority: saved enrollment and the
// exact pending operation must still be checked by continuePrepared.
func requireIndependentWorker() error {
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	var inJob uint32
	ok, _, err := kernel.NewProc("IsProcessInJob").Call(uintptr(windows.CurrentProcess()), 0, uintptr(unsafe.Pointer(&inJob)))
	if ok == 0 {
		return fmt.Errorf("inspect worker job: %w", err)
	}
	if inJob != 0 {
		return errors.New("worker is bound to a Windows Job")
	}
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
	_, err = r.continuePrepared(ctx, operation)
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
	// Do not create a key, observe/start WSL, take over a live worker, or require a
	// live registration merely to inspect a retained interrupted operation.
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Hacocoon\Reclamation\`+r.String(), registry.QUERY_VALUE)
	if err != nil {
		return PreparedStatus{}, err
	}
	defer key.Close()
	record, err := (&operationStore{key: key}).read()
	if err != nil {
		return PreparedStatus{}, err
	}
	if record.Operation != o || record.Registration.ID != r {
		return PreparedStatus{}, errors.New("saved result belongs to another operation or registration")
	}
	status := PreparedStatus{Operation: o.String(), State: record.State}
	if record.State != "pending" {
		status.Observation = &record.Observation
	}
	return status, nil
}
