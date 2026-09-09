//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

type wslOperation uint8

const (
	wslStop wslOperation = iota + 1
	wslResume
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
	default:
		return nil, errors.New("invalid managed WSL operation")
	}
}

func (r registration) runWSL(ctx context.Context, operation wslOperation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.revalidate(); err != nil {
		return err
	}
	args, err := r.wslArguments(operation)
	if err != nil {
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
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	command.WaitDelay = 5 * time.Second
	if err := command.Run(); err != nil {
		return fmt.Errorf("managed WSL operation %d: %w", operation, err)
	}
	return r.revalidate()
}

// continuationObservation distinguishes requesting systemd shutdown from proving
// native compaction completion. Resume only proves that the exact WSL can launch;
// controller/Host readiness must be checked separately by the public workflow.
type continuationObservation struct {
	StopAttempted, StopRequested bool
	Compaction                   compactObservation
	ResumeAttempted, Resumed     bool
}

// reclaimWithResume is an internal native sequence, not installation authority or
// a crash-resumption protocol. Its future public caller must durably record intent
// and results across WSL shutdown before exposing this operation to users.
func (r registration) reclaimWithResume(ctx context.Context) (result continuationObservation, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := r.revalidate(); err != nil {
		return result, err
	}
	path, err := r.diskPath()
	if err != nil {
		return result, err
	}
	pin, err := pinDisk(path)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, pin.Close()) }()
	return executeContinuation(ctx,
		func(ctx context.Context) error { return r.runWSL(ctx, wslStop) },
		pin.compact,
		func(ctx context.Context) error {
			if err := r.runWSL(ctx, wslResume); err != nil {
				return err
			}
			_, err := pin.Allocation()
			return err
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
		err = errors.Join(err, resumeErr)
	}()
	result.StopAttempted = true
	if err := stop(ctx); err != nil {
		return result, err
	}
	result.StopRequested = true
	result.Compaction, err = compact(ctx)
	return result, err
}
