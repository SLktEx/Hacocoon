package btrfs

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestMountChecksExactMountpointInsteadOfContainingFilesystem(t *testing.T) {
	var findmntArgs []string
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		switch name {
		case "findmnt":
			findmntArgs = append([]string(nil), args...)
			if containsArgument(args, "--target") {
				return host.Result{Stdout: "tmpfs\n"}, nil
			}
			if containsArgument(args, "--mountpoint") {
				return host.Result{ExitCode: 1}, errors.New("not mounted")
			}
			return host.Result{}, errors.New("missing exact mountpoint selector")
		case "mount":
			return host.Result{}, nil
		default:
			return host.Result{}, errors.New("unexpected command")
		}
	}}

	b := NewBtrfs(runner)
	if err := b.Mount(context.Background(), "/dev/loop1", "/tmp/haco/mounts/local-default"); err != nil {
		t.Fatalf("mount failed: %v", err)
	}
	want := []string{"-rn", "-o", "SOURCE", "--mountpoint", "/tmp/haco/mounts/local-default"}
	if !reflect.DeepEqual(findmntArgs, want) {
		t.Fatalf("findmnt args=%#v want=%#v", findmntArgs, want)
	}
}

func TestUnmountChecksExactMountpointInsteadOfContainingFilesystem(t *testing.T) {
	var umountCalled bool
	runner := &filesystemRunner{fn: func(name string, args []string) (host.Result, error) {
		switch name {
		case "findmnt":
			if containsArgument(args, "--target") {
				return host.Result{Stdout: "tmpfs /tmp tmpfs rw 0 0\n"}, nil
			}
			if containsArgument(args, "--mountpoint") {
				return host.Result{ExitCode: 1}, errors.New("not mounted")
			}
			return host.Result{}, errors.New("missing exact mountpoint selector")
		case "umount":
			umountCalled = true
			return host.Result{}, nil
		default:
			return host.Result{}, errors.New("unexpected command")
		}
	}}

	b := NewBtrfs(runner)
	if err := b.Unmount(context.Background(), "/tmp/haco/mounts/local-default"); err != nil {
		t.Fatalf("unmount failed: %v", err)
	}
	if umountCalled {
		t.Fatal("unmount attempted even though the exact path was not mounted")
	}
}

func containsArgument(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
