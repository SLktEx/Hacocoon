package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnsureBtrfsLoopPoolKeepsCompliantMountOptions(t *testing.T) {
	const desired = "compress=zstd:3,noatime,nodiscard"
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "storage" && args[1] == "show":
			return host.Result{}, nil
		case len(args) >= 2 && args[0] == "storage" && args[1] == "get":
			return host.Result{Stdout: desired + "\n"}, nil
		default:
			return host.Result{}, errors.New("unexpected command")
		}
	}}

	pool, err := New(runner).EnsureBtrfsLoopPool(context.Background(), BtrfsLoopPoolSpec{
		Name:         "haco-local-default",
		Size:         "128GiB",
		MountOptions: desired,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pool != "haco-local-default" {
		t.Fatalf("pool = %q", pool)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 2 && call.args[0] == "storage" && call.args[1] == "set" {
			t.Fatalf("compliant pool was rewritten: %#v", runner.calls)
		}
	}
}

func TestEnsureBtrfsLoopPoolReconcilesExistingMountOptions(t *testing.T) {
	const desired = "compress=zstd:3,noatime,nodiscard"
	current := "compress=zstd:3"
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "storage" && args[1] == "show":
			return host.Result{}, nil
		case len(args) >= 2 && args[0] == "storage" && args[1] == "get":
			return host.Result{Stdout: current + "\n"}, nil
		case len(args) >= 2 && args[0] == "storage" && args[1] == "set":
			want := "btrfs.mount_options=" + desired
			if len(args) < 4 || args[2] != "haco-local-default" || args[3] != want {
				return host.Result{ExitCode: 1}, errors.New("unexpected storage set arguments")
			}
			current = desired
			return host.Result{}, nil
		default:
			return host.Result{}, errors.New("unexpected command")
		}
	}}

	if _, err := New(runner).EnsureBtrfsLoopPool(context.Background(), BtrfsLoopPoolSpec{
		Name:         "haco-local-default",
		Size:         "128GiB",
		MountOptions: desired,
	}); err != nil {
		t.Fatal(err)
	}

	joined := make([]string, 0, len(runner.calls))
	for _, call := range runner.calls {
		joined = append(joined, strings.Join(call.args, " "))
	}
	if len(joined) != 4 || !strings.Contains(joined[2], "storage set haco-local-default btrfs.mount_options="+desired) {
		t.Fatalf("reconciliation calls = %#v", joined)
	}
}

func TestEnsureBtrfsLoopPoolSurfacesMountOptionReconcileFailure(t *testing.T) {
	const desired = "compress=zstd:3,noatime,nodiscard"
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "storage" && args[1] == "show":
			return host.Result{}, nil
		case len(args) >= 2 && args[0] == "storage" && args[1] == "get":
			return host.Result{Stdout: "compress=zstd:3\n"}, nil
		case len(args) >= 2 && args[0] == "storage" && args[1] == "set":
			return host.Result{ExitCode: 1, Stderr: "remount failed\n"}, errors.New("exit status 1")
		default:
			return host.Result{}, errors.New("unexpected command")
		}
	}}

	_, err := New(runner).EnsureBtrfsLoopPool(context.Background(), BtrfsLoopPoolSpec{
		Name:         "haco-local-default",
		Size:         "128GiB",
		MountOptions: desired,
	})
	if err == nil || !strings.Contains(err.Error(), "remount failed") {
		t.Fatalf("error = %v", err)
	}
}
