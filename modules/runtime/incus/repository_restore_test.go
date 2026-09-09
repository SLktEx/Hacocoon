package incus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

func TestSavedWorkspaceNativeCopyAndRefusal(t *testing.T) {
	for _, mode := range []string{"ok", "foreign", "busy", "missing", "legacy", "bad-idmap", "other-pool", "lost-reply", "metadata-drift"} {
		t.Run(mode, func(t *testing.T) {
			p := snapshotVolumeFixture("work")
			p.Remote = "https://github.com/example/repo.git"
			p.Branch = "main"
			p.Device = "workspace"
			p.Path = "/workspace"
			if mode == "legacy" {
				p.Remote = ""
				p.Branch = ""
			}
			v := persistentVolumeObservation{Name: p.target(), Type: "custom", ContentType: "filesystem", Config: p.targetConfig()}
			v.Config["volatile.idmap.last"] = "[]"
			if mode == "foreign" {
				v.Config["user.hacocoon.owner"] = strings.Repeat("c", 32)
			}
			if mode == "busy" {
				v.UsedBy = []string{"/1.0/instances/haco-host"}
			}
			if mode == "bad-idmap" {
				v.Config["volatile.idmap.last"] = "bad"
			}
			posts := 0
			r := New(&fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || args[0] != "query" {
					t.Fatal(name, args)
				}
				if args[1] == "-X" {
					posts++
					var req struct {
						Name   string
						Config map[string]string
						Source map[string]any
					}
					if json.Unmarshal([]byte(args[len(args)-1]), &req) != nil {
						t.Fatal("bad request")
					}
					if req.Name != "haco-work-restored" || req.Source["name"] != p.target() || req.Source["volume_only"] != true || req.Config["user.hacocoon.owner"] != strings.Repeat("d", 32) || req.Config["user.hacocoon.kind"] != "" || req.Config["volatile.idmap.last"] != "[]" {
						t.Fatal(req)
					}
					if mode == "lost-reply" {
						return host.Result{}, errors.New("lost")
					}
					return host.Result{}, nil
				}
				var value any
				if args[1] == "/1.0/storage-pools/pool" {
					value = map[string]string{"name": "pool", "driver": "btrfs"}
				} else if strings.Contains(args[1], "/volumes/custom?") {
					value = []persistentVolumeObservation{v}
					if mode == "missing" {
						value = []persistentVolumeObservation{}
					}
				} else {
					t.Fatal("consulted original Env/Base or mutated source", args)
				}
				raw, _ := json.Marshal(value)
				return host.Result{Stdout: string(raw)}, nil
			}})
			c, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: r.project, Volume: &p})
			if err != nil {
				t.Fatal(err)
			}
			c.State = "verified"
			backend := &RepositoryBackend{Runtime: r}
			sources, err := backend.SavedWorkspaces(context.Background(), core.Snapshot{State: "ready", Components: []core.SnapshotComponent{c}})
			if mode == "foreign" || mode == "busy" || mode == "missing" || mode == "legacy" {
				if err == nil || posts != 0 {
					t.Fatal("accepted invalid source", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			native, planErr := backend.PlanSavedWorkspace(context.Background(), "restored", sources[0])
			if planErr != nil || native != "pool/haco-work-restored" {
				t.Fatal("restore depended on default profile", native, planErr)
			}
			target := gitrepo.Object{Kind: "work", ID: "restored", Repository: p.SourceID, Remote: p.Remote, Branch: p.Branch, Owner: strings.Repeat("d", 32), NativeRef: "pool/haco-work-restored"}
			if mode == "other-pool" {
				target.NativeRef = "other/haco-work-restored"
			}
			if mode == "metadata-drift" {
				target.Remote = "https://github.com/example/other.git"
			}
			err = backend.CreateSavedWorkspace(context.Background(), target, sources[0])
			if mode == "ok" {
				if err != nil || posts != 1 {
					t.Fatal(err, posts)
				}
			} else {
				if err == nil {
					t.Fatal("accepted invalid copy")
				}
				if mode != "lost-reply" && posts != 0 {
					t.Fatal("unexpected mutation")
				}
			}
		})
	}
}
func TestSavedWorkspaceCleanupRequiresOwnedUnattachedPositiveAbsence(t *testing.T) {
	for _, mode := range []string{"ok", "foreign", "attached", "remains", "malformed", "snapshots", "backups", "schedule", "bad-children", "unavailable-children", "truncated-children"} {
		t.Run(mode, func(t *testing.T) {
			object := gitrepo.Object{Kind: "work", ID: "restored", Repository: "repo", Owner: strings.Repeat("d", 32), NativeRef: "pool/haco-work-restored"}
			deleted := false
			r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] == "storage" {
					deleted = true
					return host.Result{}, nil
				}
				if strings.Contains(args[1], "/snapshots?") || strings.Contains(args[1], "/backups?") {
					if mode == "bad-children" {
						return host.Result{Stdout: "null"}, nil
					}
					if mode == "unavailable-children" {
						return host.Result{ExitCode: 1}, nil
					}
					if mode == "truncated-children" {
						return host.Result{Stdout: "[]", StdoutTruncated: true}, nil
					}
					if (mode == "snapshots" && strings.Contains(args[1], "/snapshots?")) || (mode == "backups" && strings.Contains(args[1], "/backups?")) {
						return host.Result{Stdout: `["native-saved-object"]`}, nil
					}
					return host.Result{Stdout: "[]"}, nil
				}
				v := persistentVolumeObservation{Name: "haco-work-restored", Type: "custom", ContentType: "filesystem", Config: volumeConfig(object)}
				if mode == "schedule" {
					v.Config["snapshots.schedule"] = "@daily"
				}
				if mode == "foreign" {
					v.Config["user.hacocoon.owner"] = "foreign"
				}
				if mode == "attached" {
					v.UsedBy = []string{"foreign"}
				}
				list := []persistentVolumeObservation{v}
				if deleted && mode != "remains" {
					list = []persistentVolumeObservation{}
				}
				raw, _ := json.Marshal(list)
				if mode == "malformed" {
					raw = []byte("null")
				}
				return host.Result{Stdout: string(raw)}, nil
			}})
			err := (&RepositoryBackend{Runtime: r}).DeleteWorkspaceVolume(context.Background(), object)
			if mode == "ok" {
				if err != nil || !deleted {
					t.Fatal(err)
				}
			} else {
				if err == nil {
					t.Fatal("unconfirmed cleanup accepted")
				}
				if mode != "remains" && deleted {
					t.Fatal("foreign resource deleted")
				}
			}
		})
	}
}
