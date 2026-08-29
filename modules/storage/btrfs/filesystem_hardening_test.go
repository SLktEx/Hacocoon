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
