//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/reclamation"

	"golang.org/x/sys/windows"
)

type wslOperation uint8

const (
	wslStop wslOperation = iota + 1
	wslResume
	wslReadRegistration
)

// wslArguments deliberately has no caller command, shell, name lookup or
// default-distribution branch. systemd owns shutdown within the selected WSL.
func (r registration) wslArguments(operation wslOperation) ([]string, error) {
	if _, err := r.diskPath(); err != nil {
		return nil, err
	}
	args := []string{"--distribution-id", r.ID.String(), "--user", "root", "--cd", "/", "--exec"}
	switch operation {
	case wslStop:
		return append(args, "/usr/bin/env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "/usr/bin/systemctl", "--no-block", "poweroff"), nil
	case wslResume:
		return append(args, "/usr/bin/env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "/usr/bin/true"), nil
	case wslReadRegistration:
		return append(args, "/usr/bin/env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "/usr/bin/python3", "-I", "/usr/local/libexec/hacocoon-wsl-interop", "--read-registration"), nil
	default:
		return nil, errors.New("invalid managed WSL operation")
	}
}

func (r registration) runWSL(ctx context.Context, operation wslOperation) error {
	return r.runWSLTo(ctx, operation, io.Discard)
}

func (r registration) runWSLTo(ctx context.Context, operation wslOperation, output io.Writer) error {
	args, err := r.wslArguments(operation)
	if err != nil {
		return err
	}
	return r.runWSLArguments(ctx, args, output)
}

// Only fixed operation builders in this package supply arguments.
func (r registration) runWSLArguments(ctx context.Context, args []string, output io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.revalidate(); err != nil {
		return err
	}
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return err
	}
	root, err := windows.GetWindowsDirectory()
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, filepath.Join(system, "wsl.exe"), args...)
	// Do not propagate caller WSLENV, PATH, BASH_ENV, profile or terminal state.
	// SystemRoot is obtained from Windows, not from the caller environment.
	command.Env = []string{"SystemRoot=" + root, "WINDIR=" + root}
	command.Dir = system
	command.Stdin = nil
	command.Stdout = output
	command.Stderr = io.Discard
	command.WaitDelay = 5 * time.Second
	if err := command.Run(); err != nil {
		return fmt.Errorf("managed WSL operation: %w", err)
	}
	return r.revalidate()
}

// continuationObservation distinguishes requesting systemd shutdown from proving
// native compaction completion. Resume only proves that the exact WSL can launch;
// controller/Host readiness must be checked separately by the public workflow.
type continuationObservation struct {
	Failure                      string `json:",omitempty"`
	NativeError                  uint32 `json:",omitempty"`
	StopAttempted, StopRequested bool
	Compaction                   compactObservation
	ResumeAttempted, Resumed     bool
}

// withReclamationTarget holds exclusion and native pins through the entire
// action. Prepared handoff does not bypass any ordinary mutation authorization.
func (r registration) withReclamationTarget(ctx context.Context, visit func(*operationStore, *pinnedDisk, installationObservation) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.revalidate(); err != nil {
		return err
	}
	guard, err := acquireContinuation(r.ID)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, guard.Close()) }()
	path, err := r.diskPath()
	if err != nil {
		return err
	}
	pin, err := pinDisk(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, pin.Close()) }()
	records, err := openOperationStore(r.ID)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, records.close()) }()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	enrolled, err := records.readBinding()
	if err != nil {
		return fmt.Errorf("managed WSL enrollment required: %w", err)
	}
	if enrolled.Target.Registration != r || enrolled.Target.Disk != pin.identity || enrolled.Target.WindowsOwner != user.User.Sid.String() {
		return errors.New("managed WSL registration, owner or disk differs from enrollment")
	}
	identity, err := r.readInstallation(ctx)
	if err != nil {
		return err
	}
	target := installationObservation{Registration: r, Installation: identity, Disk: pin.identity, WindowsOwner: user.User.Sid.String()}
	if err := records.requireBinding(target); err != nil {
		return err
	}
	return visit(records, pin, target)
}

// prepareContinuation records one exact future execution before launching its
// Windows worker. A crash leaves the same pending record for explicit review.
func (r registration) prepareContinuation(ctx context.Context) (operationRecord, error) {
	return r.prepareContinuationVersion(ctx, 1)
}

func (r registration) prepareContinuationVersion(ctx context.Context, version int) (intent operationRecord, err error) {
	err = r.withReclamationTarget(ctx, func(records *operationStore, pin *pinnedDisk, _ installationObservation) error {
		var beginErr error
		intent, beginErr = records.beginVersion(r, pin.identity, version)
		return beginErr
	})
	return
}

// continuePrepared is only an explicit handoff of an exact operation, never a
// scan/replay of whichever interrupted record happens to exist.
func (r registration) continuePrepared(ctx context.Context, operation windows.GUID) (continuationObservation, error) {
	return r.continuePreparedReady(ctx, operation, nil)
}

func (r registration) continuePreparedReady(ctx context.Context, operation windows.GUID, ready func() error) (result continuationObservation, err error) {
	if operation == (windows.GUID{}) {
		return result, errors.New("prepared operation identity required")
	}
	err = r.withReclamationTarget(ctx, func(records *operationStore, pin *pinnedDisk, target installationObservation) error {
		intent, checkErr := records.requirePending(operation, r, pin.identity)
		if checkErr != nil {
			return checkErr
		}
		if ready != nil {
			if err := ready(); err != nil {
				return err
			}
		}
		result, checkErr = r.executeRecorded(ctx, records, pin, target, intent)
		return checkErr
	})
	return
}

// reclaimWithResume retains the synchronous internal entry for native acceptance.
// Both paths share authorization and the canonical stop/compact/resume sequence.
func (r registration) reclaimWithResume(ctx context.Context) (result continuationObservation, err error) {
	err = r.withReclamationTarget(ctx, func(records *operationStore, pin *pinnedDisk, target installationObservation) error {
		intent, beginErr := records.begin(r, pin.identity)
		if beginErr != nil {
			return beginErr
		}
		result, beginErr = r.executeRecorded(ctx, records, pin, target, intent)
		return beginErr
	})
	return
}

func (r registration) executeRecorded(ctx context.Context, records *operationStore, pin *pinnedDisk, target installationObservation, intent operationRecord) (result continuationObservation, err error) {
	return executeRecordedStages(ctx, records, intent, func(ctx context.Context) (*reclamation.LinuxReport, error) {
		if err := records.requireBinding(target); err != nil {
			return nil, err
		}
		return r.reclaimLinux(ctx, target.Installation)
	}, func(ctx context.Context) (continuationObservation, error) {
		return executeContinuation(ctx,
			func(ctx context.Context) error {
				if err := records.requireBinding(target); err != nil {
					return err
				}
				// Linux discard may take minutes. Recheck the installed identity before
				// poweroff rather than trusting the earlier observation indefinitely.
				identity, err := r.readInstallation(ctx)
				if err != nil {
					return err
				}
				if identity != target.Installation {
					return errors.New("managed installation changed before WSL stop")
				}
				return r.runWSL(ctx, wslStop)
			},
			func(ctx context.Context) (compactObservation, error) {
				if err := records.requireBinding(target); err != nil {
					return compactObservation{}, err
				}
				if err := r.revalidate(); err != nil {
					return compactObservation{}, err
				}
				return pin.compact(ctx)
			},
			func(ctx context.Context) error {
				if err := r.runWSL(ctx, wslResume); err != nil {
					return err
				}
				_, err := pin.Allocation()
				return err
			})
	})
}

// Only the native binding above supplies these actions in production. This small
// seam tests failure/cancellation order without shutting down a WSL distribution.
func executeContinuation(ctx context.Context, stop func(context.Context) error,
	compact func(context.Context) (compactObservation, error), resume func(context.Context) error,
) (result continuationObservation, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	defer func() {
		if !result.StopAttempted {
			return
		}
		resumeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		result.ResumeAttempted = true
		resumeErr := resume(resumeCtx)
		result.Resumed = resumeErr == nil
		if resumeErr != nil && result.Failure == "" {
			result.recordFailure("resume", resumeErr)
		}
		err = errors.Join(err, resumeErr)
	}()
	result.StopAttempted = true
	if err := stop(ctx); err != nil {
		result.recordFailure("stop", err)
		return result, err
	}
	result.StopRequested = true
	result.Compaction, err = compact(ctx)
	if err != nil {
		result.recordFailure("compact", err)
	}
	return result, err
}

// Persist only fixed failure categories and an optional Win32 code, never raw
// subprocess/backend errors. These fields are observations, not retry authority.
func (o *continuationObservation) recordFailure(stage string, err error) {
	o.Failure = stage
	if stage == "compact" && errors.Is(err, errVirtualDiskAttached) {
		o.Failure = "compact_attached"
	}
	var native syscall.Errno
	if errors.As(err, &native) {
		o.NativeError = uint32(native)
	}
}
