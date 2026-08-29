package localqcow2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/storage/btrfs/internal/block"
)

type Store struct {
	runner host.Runner
}

func New(runner host.Runner) *Store { return &Store{runner: runner} }

func (*Store) ID() string { return "block.local-qcow2" }

func (s *Store) Probe(ctx context.Context) (block.Capabilities, error) {
	for _, command := range []string{"qemu-img", "qemu-nbd"} {
		if _, err := s.runner.Run(ctx, command, "--version"); err != nil {
			return block.Capabilities{Available: false, Details: []string{command + " unavailable"}}, nil
		}
	}
	devices, err := filepath.Glob("/dev/nbd*")
	if err != nil {
		return block.Capabilities{}, err
	}
	if len(devices) == 0 {
		return block.Capabilities{Available: false, Details: []string{"no /dev/nbd devices; load nbd support first"}}, nil
	}
	return block.Capabilities{Available: true, Shrink: true, Compact: true}, nil
}

func (s *Store) Ensure(ctx context.Context, spec block.Spec) (block.Handle, error) {
	if err := os.MkdirAll(filepath.Dir(spec.Path), 0o700); err != nil {
		return block.Handle{}, err
	}
	_, err := os.Stat(spec.Path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if _, err := s.runner.Run(ctx, "qemu-img", "create", "-f", "qcow2", spec.Path, strconv.FormatInt(spec.SizeBytes, 10)); err != nil {
			return block.Handle{}, fmt.Errorf("create qcow2 image: %w", err)
		}
	case err != nil:
		return block.Handle{}, err
	}
	return s.Attach(ctx, block.Handle{ID: spec.ID, Path: spec.Path, Bytes: spec.SizeBytes})
}

func (s *Store) Inspect(ctx context.Context, handle block.Handle) (block.State, error) {
	result, err := s.runner.Run(ctx, "qemu-img", "info", "--output=json", handle.Path)
	if err != nil {
		return block.State{}, err
	}
	return block.State{Healthy: true, Bytes: handle.Bytes, Device: handle.Device, Details: []string{strings.TrimSpace(result.Stdout)}}, nil
}

func (s *Store) Attach(ctx context.Context, handle block.Handle) (block.Handle, error) {
	if handle.Device != "" {
		return handle, nil
	}
	device, err := findFreeNBD()
	if err != nil {
		return block.Handle{}, err
	}
	if _, err := s.runner.Run(ctx, "qemu-nbd", "--connect="+device, handle.Path); err != nil {
		return block.Handle{}, fmt.Errorf("attach qcow2 via nbd: %w", err)
	}
	handle.Device = device
	return handle, nil
}

func (s *Store) Detach(ctx context.Context, handle block.Handle) error {
	if handle.Device == "" {
		return nil
	}
	_, err := s.runner.Run(ctx, "qemu-nbd", "--disconnect", handle.Device)
	return err
}

func (s *Store) Grow(ctx context.Context, handle block.Handle, target int64) (block.Handle, error) {
	if err := s.Detach(ctx, handle); err != nil {
		return block.Handle{}, err
	}
	if _, err := s.runner.Run(ctx, "qemu-img", "resize", handle.Path, strconv.FormatInt(target, 10)); err != nil {
		return block.Handle{}, fmt.Errorf("grow qcow2: %w", err)
	}
	handle.Bytes = target
	handle.Device = ""
	return s.Attach(ctx, handle)
}

func (s *Store) Shrink(ctx context.Context, handle block.Handle, target int64) (block.Handle, error) {
	if err := s.Detach(ctx, handle); err != nil {
		return block.Handle{}, fmt.Errorf("detach before qcow2 shrink: %w", err)
	}
	if _, err := s.runner.Run(ctx, "qemu-img", "resize", "--shrink", handle.Path, strconv.FormatInt(target, 10)); err != nil {
		return block.Handle{}, fmt.Errorf("shrink qcow2: %w", err)
	}
	if _, err := s.runner.Run(ctx, "qemu-img", "check", handle.Path); err != nil {
		return block.Handle{}, fmt.Errorf("verify qcow2 after shrink: %w", err)
	}
	handle.Bytes = target
	handle.Device = ""
	return s.Attach(ctx, handle)
}

func (s *Store) Compact(ctx context.Context, handle block.Handle) error {
	if handle.Device != "" {
		return fmt.Errorf("qcow2 compact requires detached image")
	}
	_, err := s.runner.Run(ctx, "qemu-img", "check", handle.Path)
	return err
}

func (s *Store) Delete(ctx context.Context, handle block.Handle) error {
	if handle.Device == "" {
		return fmt.Errorf("refuse to delete qcow2 image %s without proven NBD device identity", handle.Path)
	}
	if err := s.Detach(ctx, handle); err != nil {
		return fmt.Errorf("disconnect qcow2 NBD device before delete: %w", err)
	}
	if err := os.Remove(handle.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func findFreeNBD() (string, error) {
	devices, err := filepath.Glob("/dev/nbd*")
	if err != nil {
		return "", err
	}
	for _, device := range devices {
		name := filepath.Base(device)
		if _, err := os.Stat(filepath.Join("/sys/block", name, "pid")); errors.Is(err, os.ErrNotExist) {
			return device, nil
		}
	}
	return "", fmt.Errorf("no free /dev/nbd device")
}
