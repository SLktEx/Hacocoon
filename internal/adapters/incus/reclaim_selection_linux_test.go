//go:build linux && (amd64 || arm64)

package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestReclaimSelectionRejectsUntrustedPoolObservations(t *testing.T) {
	const valid = `[{"name":"haco-test","driver":"btrfs","status":"Created","config":{"source":"/var/lib/incus/disks/haco-test.img","btrfs.mount_options":"compress=zstd:3,noatime,nodiscard"}}]`
	spec := BtrfsLoopPoolSpec{Name: "haco-test", MountOptions: "compress=zstd:3,noatime,nodiscard"}
	for _, tc := range []struct {
		name, body string
		exit       int
		truncated  bool
	}{
		{"missing", `[]`, 0, false},
		{"null", `null`, 0, false},
		{"malformed", `[`, 0, false},
		{"failed", valid, 1, false},
		{"truncated", valid, 0, true},
		{"foreign_source", `[{"name":"haco-test","driver":"btrfs","status":"Created","config":{"source":"/foreign/disks/haco-test.img","btrfs.mount_options":"compress=zstd:3,noatime,nodiscard"}}]`, 0, false},
		{"wrong_driver", `[{"name":"haco-test","driver":"zfs","status":"Created"}]`, 0, false},
		{"missing_policy", `[{"name":"haco-test","driver":"btrfs","status":"Created","config":{"source":"/var/lib/incus/disks/haco-test.img"}}]`, 0, false},
		{"unavailable", `[{"name":"haco-test","driver":"btrfs","status":"Unavailable"}]`, 0, false},
		{"duplicate", valid[:len(valid)-1] + "," + valid[1:], 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || !reflect.DeepEqual(args, []string{"storage", "list", "--project", sandboxResourceProject, "--format", "json"}) {
					t.Fatalf("unexpected mutation/probe: %s %v", name, args)
				}
				return host.Result{Stdout: tc.body, ExitCode: tc.exit, StdoutTruncated: tc.truncated}, nil
			}}
			op, err := New(runner).PrepareStorageReclamation(context.Background(), spec)
			if err == nil || op != nil {
				t.Fatal("accepted untrusted pool", op, err)
			}
		})
	}
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		return host.Result{Stdout: valid}, nil
	}}
	source, err := New(runner).readReclaimPool(context.Background(), spec)
	if err != nil || source != "/var/lib/incus/disks/haco-test.img" {
		t.Fatal(source, err)
	}
}

func TestReclaimSelectionCancellationAndClosedOperations(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("unexpected Incus call")
		return host.Result{}, nil
	}}
	r := New(runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.PrepareStorageReclamation(ctx, BtrfsLoopPoolSpec{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, name := range []string{"", "--help", "../other", "other/name"} {
		if _, err := r.PrepareStorageReclamation(context.Background(), BtrfsLoopPoolSpec{Name: name, MountOptions: "nodiscard"}); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(name, err)
		}
	}
	var closed *StorageReclamation
	if result, err := closed.TrimPool(context.Background()); err == nil || result.Attempted {
		t.Fatal(result, err)
	}
	if result, err := closed.TrimBackingFilesystem(context.Background()); err == nil || result.Attempted {
		t.Fatal(result, err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestReclaimRevalidatesIncusBeforeEveryStage(t *testing.T) {
	changed := errors.New("Incus correspondence changed")
	calls := 0
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		calls++
		return host.Result{}, changed
	}}
	selected := &StorageReclamation{runtime: New(runner), spec: BtrfsLoopPoolSpec{Name: "haco-test", MountOptions: "nodiscard"}, source: "/var/lib/incus/disks/haco-test.img", target: &pinnedReclaimTarget{}}
	if result, err := selected.TrimPool(context.Background()); !errors.Is(err, changed) || result.Attempted || calls != 1 {
		t.Fatal(result, err, calls)
	}
	if result, err := selected.TrimBackingFilesystem(context.Background()); !errors.Is(err, changed) || result.Attempted || calls != 2 {
		t.Fatal(result, err, calls)
	}
	if err := selected.Close(); err != nil {
		t.Fatal(err)
	}
	if result, err := selected.TrimPool(context.Background()); !errors.Is(err, core.ErrIncompatibleState) || result.Attempted || calls != 2 {
		t.Fatal(result, err, calls)
	}
}
