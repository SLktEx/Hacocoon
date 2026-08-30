package btrfs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type filesystemRunner struct {
	calls []string
	fn    func(string, []string) (host.Result, error)
}

func (r *filesystemRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	return r.fn(name, args)
}

func TestEnsureRefusesExistingNonBtrfsFilesystem(t *testing.T) {
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		if name == "blkid" {
			return host.Result{Stdout: "ext4\n"}, nil
		}
		return host.Result{}, errors.New("unexpected destructive command")
	}}
	b := NewBtrfs(runner)
	if err := b.Ensure(context.Background(), "/dev/fake", t.TempDir()); err == nil {
		t.Fatal("expected existing ext4 filesystem to be refused")
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "mkfs.btrfs ") {
			t.Fatalf("existing filesystem was formatted: %v", runner.calls)
		}
	}
}

func TestEnsureDoesNotFormatWhenProbeFailsAmbiguously(t *testing.T) {
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		if name == "blkid" {
			return host.Result{ExitCode: 1}, errors.New("I/O error")
		}
		return host.Result{}, errors.New("unexpected destructive command")
	}}
	b := NewBtrfs(runner)
	if err := b.Ensure(context.Background(), "/dev/fake", t.TempDir()); err == nil {
		t.Fatal("expected ambiguous blkid failure to stop formatting")
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "mkfs.btrfs ") {
			t.Fatalf("probe failure reached mkfs: %v", runner.calls)
		}
	}
}

func TestMountRejectsWrongExistingSource(t *testing.T) {
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		if name == "findmnt" {
			return host.Result{Stdout: "/dev/wrong\n"}, nil
		}
		return host.Result{}, errors.New("unexpected mount")
	}}
	b := NewBtrfs(runner)
	if err := b.Mount(context.Background(), "/dev/expected", "/mnt/haco"); err == nil {
		t.Fatal("expected mismatched existing mount to be rejected")
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "mount ") {
			t.Fatalf("mount attempted over mismatched source: %v", runner.calls)
		}
	}
}

func TestMountEnablesTransparentCompression(t *testing.T) {
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		if name == "findmnt" {
			return host.Result{ExitCode: 1}, errors.New("not mounted")
		}
		if name == "mount" {
			return host.Result{}, nil
		}
		return host.Result{}, errors.New("unexpected command")
	}}
	b := NewBtrfs(runner)
	if err := b.Mount(context.Background(), "/dev/expected", "/mnt/haco"); err != nil {
		t.Fatalf("mount failed: %v", err)
	}
	want := "mount -o compress=zstd:3 /dev/expected /mnt/haco"
	for _, call := range runner.calls {
		if call == want {
			return
		}
	}
	t.Fatalf("transparent compression mount option missing: %v", runner.calls)
}

func TestMountKeepsExistingExpectedCompression(t *testing.T) {
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		if name != "findmnt" || len(args) < 3 {
			return host.Result{}, errors.New("unexpected command")
		}
		switch args[2] {
		case "SOURCE":
			return host.Result{Stdout: "/dev/expected\n"}, nil
		case "OPTIONS":
			return host.Result{Stdout: "rw,relatime,compress=zstd:3,space_cache=v2\n"}, nil
		default:
			return host.Result{}, errors.New("unexpected findmnt column")
		}
	}}
	b := NewBtrfs(runner)
	if err := b.Mount(context.Background(), "/dev/expected", "/mnt/haco"); err != nil {
		t.Fatalf("reuse failed: %v", err)
	}
	for _, call := range runner.calls {
		if strings.HasPrefix(call, "mount ") {
			t.Fatalf("already compliant mount was remounted: %v", runner.calls)
		}
	}
}

func TestMountRemountsExistingSourceWithoutCompression(t *testing.T) {
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		if name == "findmnt" && len(args) >= 3 {
			switch args[2] {
			case "SOURCE":
				return host.Result{Stdout: "/dev/expected\n"}, nil
			case "OPTIONS":
				return host.Result{Stdout: "rw,relatime,space_cache=v2\n"}, nil
			}
		}
		if name == "mount" {
			return host.Result{}, nil
		}
		return host.Result{}, errors.New("unexpected command")
	}}
	b := NewBtrfs(runner)
	if err := b.Mount(context.Background(), "/dev/expected", "/mnt/haco"); err != nil {
		t.Fatalf("remount failed: %v", err)
	}
	want := "mount -o remount,compress=zstd:3 /dev/expected /mnt/haco"
	for _, call := range runner.calls {
		if call == want {
			return
		}
	}
	t.Fatalf("existing mount was not remounted with compression: %v", runner.calls)
}

func TestMountReplacesCompressForce(t *testing.T) {
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		if name == "findmnt" && len(args) >= 3 {
			switch args[2] {
			case "SOURCE":
				return host.Result{Stdout: "/dev/expected\n"}, nil
			case "OPTIONS":
				return host.Result{Stdout: "rw,compress-force=zstd:3,space_cache=v2\n"}, nil
			}
		}
		if name == "mount" {
			return host.Result{}, nil
		}
		return host.Result{}, errors.New("unexpected command")
	}}
	b := NewBtrfs(runner)
	if err := b.Mount(context.Background(), "/dev/expected", "/mnt/haco"); err != nil {
		t.Fatalf("remount failed: %v", err)
	}
	want := "mount -o remount,compress=zstd:3 /dev/expected /mnt/haco"
	for _, call := range runner.calls {
		if call == want {
			return
		}
	}
	t.Fatalf("compress-force mount was not replaced: %v", runner.calls)
}
