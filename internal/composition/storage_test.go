package composition

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type storageCall struct {
	name string
	args []string
}

type storageRunnerFunc struct {
	calls []storageCall
	run   func(name string, args []string) (host.Result, error)
}

func (r *storageRunnerFunc) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	copied := append([]string(nil), args...)
	r.calls = append(r.calls, storageCall{name: name, args: copied})
	return r.run(name, copied)
}

func TestDefaultIncusStorageAttachmentIsIncusOwned(t *testing.T) {
	attachment := defaultIncusStorageAttachment()
	if attachment["incus_pool"] != "haco-local-default" {
		t.Fatalf("incus_pool = %q", attachment["incus_pool"])
	}
	if attachment["driver"] != "btrfs" {
		t.Fatalf("driver = %q", attachment["driver"])
	}
	if attachment["size"] != "128GiB" {
		t.Fatalf("size = %q", attachment["size"])
	}
	if attachment["btrfs.mount_options"] != "compress=zstd:3,noatime,nodiscard" {
		t.Fatalf("btrfs.mount_options = %q", attachment["btrfs.mount_options"])
	}
	if source := attachment["source"]; source != "" {
		t.Fatalf("default attachment must not supply Host source path, got %q", source)
	}
	for _, want := range []string{"compress=zstd:3", "noatime", "nodiscard"} {
		if !strings.Contains(attachment["btrfs.mount_options"], want) {
			t.Fatalf("mount policy %q missing %q", attachment["btrfs.mount_options"], want)
		}
	}
	if strings.Contains(attachment["btrfs.mount_options"], "autodefrag") {
		t.Fatalf("autodefrag must remain disabled: %q", attachment["btrfs.mount_options"])
	}
	if strings.Contains(attachment["btrfs.mount_options"], "compress-force") {
		t.Fatalf("compress-force must remain disabled: %q", attachment["btrfs.mount_options"])
	}
}

func TestEnsureDefaultIncusStoragePoolCreatesLoopBackedPool(t *testing.T) {
	created := false
	mountOptions := ""
	runner := &storageRunnerFunc{run: func(_ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "storage" && args[1] == "show" {
			if !created {
				return host.Result{ExitCode: 1}, errors.New("not found")
			}
			return host.Result{}, nil
		}
		if len(args) >= 2 && args[0] == "storage" && args[1] == "create" {
			created = true
			for _, arg := range args {
				if strings.HasPrefix(arg, "btrfs.mount_options=") {
					mountOptions = strings.TrimPrefix(arg, "btrfs.mount_options=")
				}
			}
			return host.Result{}, nil
		}
		if len(args) >= 2 && args[0] == "storage" && args[1] == "get" {
			return host.Result{Stdout: mountOptions + "\n"}, nil
		}
		return host.Result{}, nil
	}}

	attachment, err := ensureDefaultIncusStoragePool(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(attachment, map[string]string{"incus_pool": "haco-local-default"}) {
		t.Fatalf("attachment = %#v", attachment)
	}
	wantCreate := []string{
		"storage", "create", "haco-local-default", "btrfs",
		"size=128GiB",
		"btrfs.mount_options=compress=zstd:3,noatime,nodiscard",
		"--project", "hacocoon",
	}
	if len(runner.calls) != 4 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	if runner.calls[1].name != "incus" || !reflect.DeepEqual(runner.calls[1].args, wantCreate) {
		t.Fatalf("create call = %#v, want incus %#v", runner.calls[1], wantCreate)
	}
	for _, arg := range runner.calls[1].args {
		if strings.HasPrefix(arg, "source=") {
			t.Fatalf("Incus-owned loop pool unexpectedly specifies source: %#v", runner.calls[1].args)
		}
	}
	if runner.calls[2].args[0] != "storage" || runner.calls[2].args[1] != "show" {
		t.Fatalf("post-create verification missing: %#v", runner.calls)
	}
	if runner.calls[3].args[0] != "storage" || runner.calls[3].args[1] != "get" {
		t.Fatalf("mount-policy verification missing: %#v", runner.calls)
	}
}

func TestEnsureDefaultIncusStoragePoolReusesMatchingExistingPool(t *testing.T) {
	runner := &storageRunnerFunc{run: func(_ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "storage" && args[1] == "get" {
			return host.Result{Stdout: "compress=zstd:3,noatime,nodiscard\n"}, nil
		}
		return host.Result{}, nil
	}}
	attachment, err := ensureDefaultIncusStoragePool(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if attachment["incus_pool"] != "haco-local-default" {
		t.Fatalf("attachment = %#v", attachment)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("unexpected calls = %#v", runner.calls)
	}
	if runner.calls[0].args[0] != "storage" || runner.calls[0].args[1] != "show" || runner.calls[1].args[1] != "get" {
		t.Fatalf("unexpected calls = %#v", runner.calls)
	}
}

func TestEnsureDefaultIncusStoragePoolReconcilesExistingMountPolicy(t *testing.T) {
	mountOptions := "compress=zstd:3"
	runner := &storageRunnerFunc{run: func(_ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "storage" && args[1] == "get":
			return host.Result{Stdout: mountOptions + "\n"}, nil
		case len(args) >= 2 && args[0] == "storage" && args[1] == "set":
			mountOptions = strings.TrimPrefix(args[3], "btrfs.mount_options=")
			return host.Result{}, nil
		default:
			return host.Result{}, nil
		}
	}}

	if _, err := ensureDefaultIncusStoragePool(context.Background(), runner); err != nil {
		t.Fatal(err)
	}
	if mountOptions != "compress=zstd:3,noatime,nodiscard" {
		t.Fatalf("mount options = %q", mountOptions)
	}
	wantSet := []string{
		"storage", "set", "haco-local-default",
		"btrfs.mount_options=compress=zstd:3,noatime,nodiscard",
		"--project", "hacocoon",
	}
	if len(runner.calls) != 4 || !reflect.DeepEqual(runner.calls[2].args, wantSet) {
		t.Fatalf("calls = %#v, want reconcile %#v", runner.calls, wantSet)
	}
}

func TestEnsureDefaultIncusStoragePoolSurfacesCreateStderr(t *testing.T) {
	runner := &storageRunnerFunc{run: func(_ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "storage" && args[1] == "show" {
			return host.Result{ExitCode: 1}, errors.New("not found")
		}
		return host.Result{ExitCode: 1, Stderr: "Error: loop pool creation failed\n"}, errors.New("exit status 1")
	}}
	_, err := ensureDefaultIncusStoragePool(context.Background(), runner)
	if err == nil {
		t.Fatal("expected create failure")
	}
	for _, want := range []string{"haco-local-default", "loop pool creation failed", "exit status 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error = %q, want %q", err, want)
		}
	}
}
