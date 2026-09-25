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

func TestSourceDeletionBypassesPreflightBlockersAfterReview(t *testing.T) {
	for _, mode := range []string{"foreign owner", "changed device", "foreign user", "snapshots", "backups", "schedule", "pending copy", "lost detach response", "lost delete response"} {
		t.Run(mode, func(t *testing.T) {
			service, _, native, object, _ := sourceDeleteCatalog(t, mode)
			if err := service.DeleteSource(context.Background(), object.ID, object.Owner); err != nil {
				t.Fatalf("reviewed deletion was blocked by %q: %v", mode, err)
			}
			if native.present || native.attached {
				t.Fatalf("reviewed deletion did not remove native source: attached=%t present=%t", native.attached, native.present)
			}
			if _, err := service.Get("repo", object.ID); !errors.Is(err, core.ErrNotFound) {
				t.Fatalf("successful deletion retained source record: %v", err)
			}
			assertSourceDeletionRetainedData(t, service.Root)
		})
	}
}

func TestSourceDeletionFailsOnlyWhenNativeAbsenceCannotBeReachedOrConfirmed(t *testing.T) {
	for _, mode := range []string{"still attached", "still present", "unconfirmed absence", "truncated inventory"} {
		t.Run(mode, func(t *testing.T) {
			service, _, native, object, original := sourceDeleteCatalog(t, mode)
			if err := service.DeleteSource(context.Background(), object.ID, object.Owner); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatalf("unremoved/unconfirmed native source did not fail: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(service.Root, "repo-source.json"))
			if err != nil || !bytes.Equal(data, original) {
				t.Fatal("failed deletion did not retain its exact retry record")
			}
			assertSourceDeletionRetainedData(t, service.Root)

			// Once the provider can actually remove and confirm the native target,
			// the same command succeeds without a separate recovery state.
			native.mode = ""
			if err := service.DeleteSource(context.Background(), object.ID, object.Owner); err != nil {
				t.Fatal("retry after native blocker cleared failed", err)
			}
			if native.present || native.attached {
				t.Fatal("retry did not converge on native absence")
			}
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
	err = backend.ForceDeleteSourceVolume(ctx, object)
	if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, core.ErrStorageBusy) || native.detachCalls != 0 || native.deleteCalls != 0 || !native.attached || !native.present {
		t.Fatalf("busy Host lifecycle allowed source mutation: %v", err)
	}
}
