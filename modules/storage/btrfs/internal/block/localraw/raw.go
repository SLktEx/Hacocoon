package localraw

import (
	"bufio"
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

func (*Store) ID() string { return "block.local-raw" }

func (s *Store) Probe(ctx context.Context) (block.Capabilities, error) {
	checks := []struct {
		name string
		args []string
	}{
		{"losetup", []string{"--version"}},
		{"truncate", []string{"--version"}},
	}
	for _, check := range checks {
		if _, err := s.runner.Run(ctx, check.name, check.args...); err != nil {
			return block.Capabilities{Available: false, Details: []string{check.name + " unavailable"}}, nil
		}
	}
	return block.Capabilities{Available: true, Shrink: true, Compact: true}, nil
}

func (s *Store) Ensure(ctx context.Context, spec block.Spec) (block.Handle, error) {
	if err := os.MkdirAll(filepath.Dir(spec.Path), 0o700); err != nil {
		return block.Handle{}, err
	}
	info, err := os.Stat(spec.Path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if _, err := s.runner.Run(ctx, "truncate", "-s", strconv.FormatInt(spec.SizeBytes, 10), spec.Path); err != nil {
			return block.Handle{}, fmt.Errorf("create sparse raw image: %w", err)
		}
	case err != nil:
		return block.Handle{}, err
	case info.Size() < spec.SizeBytes:
		if _, err := s.runner.Run(ctx, "truncate", "-s", strconv.FormatInt(spec.SizeBytes, 10), spec.Path); err != nil {
			return block.Handle{}, fmt.Errorf("grow sparse raw image: %w", err)
		}
	}
	return s.Attach(ctx, block.Handle{ID: spec.ID, Path: spec.Path, Bytes: spec.SizeBytes})
}

func (s *Store) Inspect(ctx context.Context, handle block.Handle) (block.State, error) {
	info, err := os.Stat(handle.Path)
	if err != nil {
		return block.State{}, err
	}
	device, err := s.findDevice(ctx, handle.Path)
	if err != nil {
		return block.State{}, fmt.Errorf("inspect raw loop device: %w", err)
	}
	return block.State{Healthy: true, Bytes: info.Size(), Device: device}, nil
}

func (s *Store) Attach(ctx context.Context, handle block.Handle) (block.Handle, error) {
	device, err := s.findDevice(ctx, handle.Path)
	if err != nil {
		return block.Handle{}, fmt.Errorf("inspect existing raw loop device: %w", err)
	}
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
	device := handle.Device
	if device == "" {
		var err error
		device, err = s.findDevice(ctx, handle.Path)
		if err != nil {
			return fmt.Errorf("inspect raw loop device before detach: %w", err)
		}
	}
	if device == "" {
		return nil
	}
	_, err := s.runner.Run(ctx, "losetup", "-d", device)
	return err
}

func (s *Store) Grow(ctx context.Context, handle block.Handle, target int64) (block.Handle, error) {
	if _, err := s.runner.Run(ctx, "truncate", "-s", strconv.FormatInt(target, 10), handle.Path); err != nil {
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
	if err := s.Detach(ctx, handle); err != nil {
		return block.Handle{}, fmt.Errorf("detach before raw shrink: %w", err)
	}
	if _, err := s.runner.Run(ctx, "truncate", "-s", strconv.FormatInt(target, 10), handle.Path); err != nil {
		return block.Handle{}, fmt.Errorf("shrink raw image: %w", err)
	}
	handle.Bytes = target
	handle.Device = ""
	return s.Attach(ctx, handle)
}

func (s *Store) Compact(ctx context.Context, handle block.Handle) error {
	_, err := s.runner.Run(ctx, "fallocate", "-d", handle.Path)
	return err
}

func (s *Store) Delete(ctx context.Context, handle block.Handle) error {
	if err := s.Detach(ctx, handle); err != nil {
		return fmt.Errorf("detach raw image before delete: %w", err)
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
