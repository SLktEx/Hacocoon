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

	"github.com/SLktEx/Hacocoon/internal/platform/wsl/coord"
	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"

	"golang.org/x/sys/windows"
)

type wslOperation uint8

const (
	wslResume wslOperation = iota + 1
	wslReadRegistration
)

// wslArguments deliberately has no caller command, shell, name lookup or
// default-distribution branch. Operations here address the enrolled GUID.
func (r registration) wslArguments(operation wslOperation) ([]string, error) {
	if _, err := r.diskPath(); err != nil {
		return nil, err
	}
	args := []string{"--distribution-id", r.ID.String(), "--user", "root", "--cd", "/", "--exec"}
	switch operation {
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

// managedCompactArguments uses WSL's public per-distribution compaction entry.
// The public CLI accepts a distribution name, while Hacocoon separately pins and
// revalidates the enrolled GUID, disk identity and Windows owner before and after
// invocation. WSL resolves this name to a GUID before its compact operation.
func (r registration) managedCompactArguments() ([]string, error) {
	if _, err := r.diskPath(); err != nil {
		return nil, err
	}
	return []string{"--manage", r.Name, "--compact"}, nil
}

func (r registration) runWSLManagedCompact(ctx context.Context) error {
	args, err := r.managedCompactArguments()
	if err != nil {
		return err
	}
	return r.runWSLArguments(ctx, args, io.Discard)
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

// continuationObservation distinguishes requesting WSL-managed compaction from
// proving compaction completion. The managed operation terminates only the exact
// selected distribution before compacting it. Resume only proves that the exact
// WSL can launch; controller/Host readiness is checked by the public workflow.
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
		return targetStage("registration", err)
	}
	guard, err := acquireContinuation(r.ID)
	if err != nil {
		return targetStage("exclusion", err)
	}
	defer func() { err = errors.Join(err, guard.Close()) }()
	// Keep native notification peers from reopening this WSL between stop and
	// compaction/resume. The registration/disk checks remain mutation authority.
	launchGuard, err := wslcoord.AcquireLaunch(r.Name)
	if err != nil {
		return targetStage("exclusion", err)
	}
	defer func() { err = errors.Join(err, launchGuard.Close()) }()
	path, err := r.diskPath()
	if err != nil {
		return targetStage("disk_path", err)
	}
	pin, err := pinDisk(path)
	if err != nil {
		return targetStage("disk_access", err)
	}
	defer func() { err = errors.Join(err, pin.Close()) }()
	records, err := openOperationStore(r.ID)
	if err != nil {
		return targetStage("record_access", err)
	}
	defer func() { err = errors.Join(err, records.close()) }()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return targetStage("windows_owner", err)
	}
	enrolled, err := records.readBinding()
	if err != nil {
		return targetStage("enrollment", err)
	}
	if enrolled.Target.Registration != r || enrolled.Target.Disk != pin.identity || enrolled.Target.WindowsOwner != user.User.Sid.String() {
		return targetStage("binding", errors.New("managed WSL registration, owner or disk differs from enrollment"))
	}
	identity, err := r.readInstallation(ctx)
	if err != nil {
		return targetStage("installation", err)
	}
	target := installationObservation{Registration: r, Installation: identity, Disk: pin.identity, WindowsOwner: user.User.Sid.String()}
	if err := records.requireBinding(target); err != nil {
		return targetStage("binding", err)
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
		return targetStage("intent", beginErr)
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
		return executeManagedContinuation(ctx,
			func(ctx context.Context) (compactObservation, error) {
				if err := records.requireBinding(target); err != nil {
					return compactObservation{}, err
				}
				// Linux discard may take minutes. Recheck the installed identity immediately
				// before handing stop+offline trim+compaction to the WSL service.
				identity, err := r.readInstallation(ctx)
				if err != nil {
					return compactObservation{}, err
				}
				if identity != target.Installation {
					return compactObservation{}, errors.New("managed installation changed before WSL compaction")
				}
				if err := r.revalidate(); err != nil {
					return compactObservation{}, err
				}
				return pin.compactManaged(ctx, r)
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

// executeManagedContinuation models the public WSL compact operation as one
// stop+compact request. A failed preflight does not launch or resume WSL; once the
// command is attempted, bounded resume is always tried because WSL may have
// terminated the selected distribution before reporting an error.
func executeManagedContinuation(ctx context.Context,
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
	result.Compaction, err = compact(ctx)
	result.StopAttempted = result.Compaction.Attempted
	result.StopRequested = result.Compaction.Attempted
	if err != nil {
		result.recordFailure("compact", err)
	}
	return result, err
}

// The legacy native sequencing seam remains for component tests of partial
// stop/compact/resume failure ordering. Production uses WSL-managed compaction.
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
