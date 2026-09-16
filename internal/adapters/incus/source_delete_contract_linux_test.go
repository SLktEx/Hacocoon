//go:build linux

package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// The native observation shapes match TestRealIncusSourceDeletionE2E. Only
// provider operations are simulated; registry persistence/reopening and the
// adapter's Host lifecycle lock run normally. This is not native acceptance.
type sourceDeleteNative struct {
	t                        *testing.T
	mode                     string
	project                  string
	owner                    string
	attached, present        bool
	detachCalls, deleteCalls int
}

func (n *sourceDeleteNative) result(value any) (host.Result, error) {
	data, err := json.Marshal(value)
	return host.Result{Stdout: string(data)}, err
}

func (n *sourceDeleteNative) Run(_ context.Context, command string, args ...string) (host.Result, error) {
	if command != "incus" {
		n.t.Fatalf("unexpected provider %q", command)
	}
	if reflect.DeepEqual(args, []string{"config", "get", "haco-host", "user.hacocoon.oci-copy", "--project", n.project}) {
		if n.mode == "pending copy" {
			return host.Result{Stdout: "retained copy receipt"}, nil
		}
		return host.Result{}, nil
	}
	if reflect.DeepEqual(args, []string{"config", "device", "remove", "haco-host", "haco-repo-source", "--project", n.project}) {
		n.detachCalls++
		if n.mode != "still attached" {
			n.attached = false
		}
		if n.mode == "lost detach response" {
			return host.Result{ExitCode: 1}, nil
		}
		return host.Result{}, nil
	}
	if reflect.DeepEqual(args, []string{"storage", "volume", "delete", "pool", "haco-repo-source", "--project", n.project}) {
		n.deleteCalls++
		if n.attached || !n.present {
			n.t.Fatal("deleted an attached or already absent volume")
		}
		if n.mode != "still present" {
			n.present = false
		}
		if n.mode == "lost delete response" {
			return host.Result{ExitCode: 1}, nil
		}
		return host.Result{}, nil
	}
	if len(args) != 2 || args[0] != "query" {
		n.t.Fatalf("unexpected mutation: %q", args)
	}
	switch args[1] {
	case "/1.0/instances/haco-host?project=" + n.project:
		devices := map[string]map[string]string{"unrelated": {"type": "disk", "path": "/keep", "pool": "pool", "source": "keep"}}
		if n.attached {
			devices["haco-repo-source"] = map[string]string{"type": "disk", "pool": "pool", "source": "haco-repo-source", "path": "/var/lib/hacocoon-repos/source"}
			if n.mode == "changed device" {
				devices["haco-repo-source"]["source"] = "foreign"
			}
		}
		return n.result(map[string]any{"name": "haco-host", "type": "container", "config": map[string]string{"user.hacocoon.role": "trusted-host"}, "devices": devices, "expanded_devices": devices})
	case "/1.0/storage-pools/pool/volumes/custom?project=" + n.project + "&recursion=1":
		if n.mode == "truncated inventory" || n.mode == "unconfirmed absence" && !n.present {
			return host.Result{Stdout: "[]", StdoutTruncated: true}, nil
		}
		volumes := []map[string]any{{"name": "keep", "type": "custom", "content_type": "filesystem", "config": map[string]string{"user.hacocoon.owner": "another-owner"}}}
		if n.present {
			config := map[string]string{"user.hacocoon.owner": n.owner, "user.hacocoon.role": "repo", "user.hacocoon.repository": "source"}
			if n.mode == "foreign owner" {
				config["user.hacocoon.owner"] = strings.Repeat("b", 32)
			}
			if n.mode == "schedule" {
				config["snapshots.schedule"] = "@daily"
			}
			users := []string{}
			if n.attached {
				users = append(users, "/1.0/instances/haco-host?project="+n.project)
			}
			if n.mode == "foreign user" || n.mode == "user after detach" && !n.attached {
				users = append(users, "/1.0/instances/other?project="+n.project)
			}
			volumes = append(volumes, map[string]any{"name": "haco-repo-source", "type": "custom", "content_type": "filesystem", "config": config, "used_by": users})
		}
		return n.result(volumes)
	case "/1.0/storage-pools/pool/volumes/custom/haco-repo-source/snapshots?project=" + n.project,
		"/1.0/storage-pools/pool/volumes/custom/haco-repo-source/backups?project=" + n.project:
		if n.mode == "snapshots" && strings.Contains(args[1], "/snapshots?") || n.mode == "backups" && strings.Contains(args[1], "/backups?") {
			return n.result([]string{"saved-child"})
		}
		return n.result([]string{})
	default:
		n.t.Fatalf("unexpected provider observation: %q", args)
		return host.Result{}, errors.New("unexpected observation")
	}
}

func sourceDeleteCatalog(t *testing.T, mode string) (*gitrepo.RepositoryService, *RepositoryBackend, *sourceDeleteNative, gitrepo.Object, []byte) {
	t.Helper()
	root := t.TempDir()
	object := gitrepo.Object{Kind: "repo", ID: "source", Repository: "source", Remote: "https://github.com/example/source.git", Branch: "main", NativeRef: "pool/haco-repo-source", Owner: strings.Repeat("a", 32), State: "ready"}
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "repo-source.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "retained-note"), []byte("independent retained data"), 0600); err != nil {
		t.Fatal(err)
	}
	native := &sourceDeleteNative{t: t, mode: mode, project: "haco-delete-contract", owner: object.Owner, attached: true, present: true}
	r := New(native)
	r.project = native.project
	backend := &RepositoryBackend{Runtime: r}
	return gitrepo.NewRepositoryService(root, backend), backend, native, object, data
}

func assertSourceDeletionRetainedData(t *testing.T, root string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "retained-note"))
	if err != nil || string(data) != "independent retained data" {
		t.Fatal("deletion changed unrelated retained data")
	}
}

func TestSourceDeletionPreflightPreservesReadyCatalogAndAttachment(t *testing.T) {
	for _, mode := range []string{"foreign owner", "changed device", "foreign user", "snapshots", "backups", "schedule", "truncated inventory"} {
		t.Run(mode, func(t *testing.T) {
			service, _, native, object, original := sourceDeleteCatalog(t, mode)
			want := core.ErrStorageBusy
			switch mode {
			case "foreign owner", "changed device":
				want = core.ErrCapabilityStale
			case "truncated inventory":
				want = core.ErrRuntimeUnavailable
			}
			if err := service.DeleteSource(context.Background(), object.ID, object.Owner); !errors.Is(err, want) {
				t.Fatalf("source refusal = %v, want %v", err, want)
			}
			data, err := os.ReadFile(filepath.Join(service.Root, "repo-source.json"))
			if err != nil || !bytes.Equal(data, original) || !native.present || !native.attached || native.detachCalls != 0 || native.deleteCalls != 0 {
				t.Fatal("preflight refusal changed source ownership or native data")
			}
			assertSourceDeletionRetainedData(t, service.Root)
			native.mode = ""
			if err := service.DeleteSource(context.Background(), object.ID, object.Owner); err != nil || native.present || native.attached {
				t.Fatalf("explicit retry after clearing blocker failed: %v", err)
			}
			assertSourceDeletionRetainedData(t, service.Root)
		})
	}
}

func TestSourceDeletionRetainsExactRecoveryReceiptUntilConfirmedAbsence(t *testing.T) {
	for _, mode := range []string{"pending copy", "lost detach response", "still attached", "user after detach", "lost delete response", "still present", "unconfirmed absence"} {
		t.Run(mode, func(t *testing.T) {
			service, backend, native, object, _ := sourceDeleteCatalog(t, mode)
			if err := service.DeleteSource(context.Background(), object.ID, object.Owner); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatalf("partial cleanup lost recovery status: %v", err)
			}
			// A fresh service instance must recover exactly the original owner
			// and native target; no in-memory state is used for the retry.
			reopened := gitrepo.NewRepositoryService(service.Root, backend)
			retained, err := reopened.Get("repo", object.ID)
			want := object
			want.State = "deleting"
			if !errors.Is(err, core.ErrRecoveryRequired) || !reflect.DeepEqual(retained, want) {
				t.Fatalf("lost exact cleanup receipt: %+v, %v", retained, err)
			}
			if (mode == "pending copy" || mode == "still attached" || mode == "lost detach response" || mode == "user after detach") && (native.deleteCalls != 0 || !native.present) {
				t.Fatal("volume was deleted before attachment/operation blockers cleared")
			}
			assertSourceDeletionRetainedData(t, service.Root)
			// The test operator clears the external failure, then explicitly
			// retries the ordinary command. The implementation does not repair it.
			native.mode = ""
			if err := reopened.DeleteSource(context.Background(), object.ID, object.Owner); err != nil {
				t.Fatal(err)
			}
			if native.present || native.attached || native.deleteCalls > 2 || native.detachCalls > 2 {
				t.Fatal("retry failed to converge on owned native absence")
			}
			if _, err := reopened.Get("repo", object.ID); !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("confirmed deletion retained catalog: %v", err)
			}
			assertSourceDeletionRetainedData(t, service.Root)
		})
	}
}

func TestSourceDeletionWaitsForHostLifecycleLockWithoutMutating(t *testing.T) {
	_, backend, native, object, _ := sourceDeleteCatalog(t, "")
	unlock, err := lockHostOperation(context.Background(), native.project)
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	err = backend.DeleteSourceVolume(ctx, object)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, core.ErrStorageBusy) || native.detachCalls != 0 || native.deleteCalls != 0 || !native.attached || !native.present {
		t.Fatalf("busy Host lifecycle allowed source mutation: %v", err)
	}
}
