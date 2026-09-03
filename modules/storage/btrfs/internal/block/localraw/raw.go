package localraw

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type Store struct {
	runner host.Runner
}

func New(runner host.Runner) *Store { return &Store{runner: runner} }

func (*Store) ID() string { return "block.local-raw" }

func (s *Store) Probe(ctx context.Context) (block.Capabilities, error) {
	if _, err := s.runner.Run(ctx, "losetup", "--version"); err != nil {
		return block.Capabilities{Available: false, Details: []string{"losetup unavailable"}}, nil
	}
	return block.Capabilities{Available: true, Shrink: true, Compact: true}, nil
}

func (s *Store) Ensure(ctx context.Context, spec block.Spec) (block.Handle, error) {
	if err := block.PrepareBackingDirectory(spec.Path); err != nil {
		return block.Handle{}, fmt.Errorf("prepare raw backing directory: %w", err)
	}
	info, err := block.ValidateBackingPath(spec.Path, true)
	if err != nil {
		return block.Handle{}, err
	}
	switch {
	case info == nil:
		if err := resizeRawNoFollow(spec.Path, spec.SizeBytes, true); err != nil {
			return block.Handle{}, fmt.Errorf("create sparse raw image: %w", err)
		}
		if _, err := block.ValidateBackingPath(spec.Path, false); err != nil {
			return block.Handle{}, fmt.Errorf("validate created raw image: %w", err)
		}
	case info.Size() < spec.SizeBytes:
		if err := resizeRawNoFollow(spec.Path, spec.SizeBytes, false); err != nil {
			return block.Handle{}, fmt.Errorf("grow sparse raw image: %w", err)
		}
	}
	return s.Attach(ctx, block.Handle{ID: spec.ID, Path: spec.Path, Bytes: spec.SizeBytes})
}

func (s *Store) Inspect(ctx context.Context, handle block.Handle) (block.State, error) {
	info, err := block.ValidateBackingPath(handle.Path, false)
	if err != nil {
		return block.State{}, err
	}
	device, _ := s.findDevice(ctx, handle.Path)
	return block.State{Healthy: true, Bytes: info.Size(), Device: device}, nil
}

func (s *Store) Attach(ctx context.Context, handle block.Handle) (block.Handle, error) {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return block.Handle{}, err
	}
	device, _ := s.findDevice(ctx, handle.Path)
	if device != "" {
		handle.Device = device
		return handle, nil
	}
	result, err := s.runner.Run(ctx, "losetup", "--find", "--show", handle.Path)
	if err != nil {
		return block.Handle{}, fmt.Errorf("attach raw loop image: %w", err)
	}
	handle.Device = strings.TrimSpace(result.Stdout)
	return handle, nil
}

func (s *Store) Detach(ctx context.Context, handle block.Handle) error {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return err
	}
	// A loop device name is only an ephemeral attachment. It can be reused for a
	// different backing file after detach or reboot, so never trust Handle.Device
	// for a destructive operation. Resolve the current attachment from the
	// authoritative backing path immediately before detaching.
	device, err := s.findDevice(ctx, handle.Path)
	if err != nil {
		return fmt.Errorf("discover raw loop image before detach: %w", err)
	}
	if device == "" {
		return nil
	}
	_, err = s.runner.Run(ctx, "losetup", "-d", device)
	return err
}

func (s *Store) Grow(ctx context.Context, handle block.Handle, target int64) (block.Handle, error) {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return block.Handle{}, err
	}
	if err := resizeRawNoFollow(handle.Path, target, false); err != nil {
		return block.Handle{}, fmt.Errorf("grow raw image: %w", err)
	}
	attached, err := s.Attach(ctx, handle)
	if err != nil {
		return block.Handle{}, err
	}
	if _, err := s.runner.Run(ctx, "losetup", "-c", attached.Device); err != nil {
		return block.Handle{}, fmt.Errorf("rescan loop device: %w", err)
	}
	attached.Bytes = target
	return attached, nil
}

func (s *Store) Shrink(ctx context.Context, handle block.Handle, target int64) (block.Handle, error) {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return block.Handle{}, err
	}
	if err := s.Detach(ctx, handle); err != nil {
		return block.Handle{}, fmt.Errorf("detach before raw shrink: %w", err)
	}
	if err := resizeRawNoFollow(handle.Path, target, false); err != nil {
		return block.Handle{}, fmt.Errorf("shrink raw image: %w", err)
	}
	handle.Bytes = target
	handle.Device = ""
	return s.Attach(ctx, handle)
}

func (s *Store) Compact(ctx context.Context, handle block.Handle) error {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		return err
	}
	_, err := s.runner.Run(ctx, "fallocate", "-d", handle.Path)
	return err
}

func (s *Store) Delete(ctx context.Context, handle block.Handle) error {
	if _, err := block.ValidateBackingPath(handle.Path, false); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := s.Detach(ctx, handle); err != nil {
		return fmt.Errorf("detach raw loop image before delete: %w", err)
	}
	if err := os.Remove(handle.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Store) findDevice(ctx context.Context, path string) (string, error) {
	result, err := s.runner.Run(ctx, "losetup", "-j", path)
	if err != nil {
		return "", err
	}
	scanner := bufio.NewScanner(strings.NewReader(result.Stdout))
	if !scanner.Scan() {
		return "", nil
	}
	line := scanner.Text()
	device, _, ok := strings.Cut(line, ":")
	if !ok {
		return "", nil
	}
	return strings.TrimSpace(device), nil
}

func resizeRawNoFollow(path string, size int64, create bool) error {
	flags := syscall.O_RDWR | syscall.O_NOFOLLOW | syscall.O_CLOEXEC
	if create {
		flags |= syscall.O_CREAT | syscall.O_EXCL
	}
	fd, err := syscall.Open(path, flags, 0o600)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = syscall.Close(fd)
		return fmt.Errorf("open raw image %q", path)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("raw backing image %q must be a regular file", path)
	}
	return file.Truncate(size)
}
