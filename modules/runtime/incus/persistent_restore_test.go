package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestSavedOCIUsesOwnedSamePoolCopy(t *testing.T) {
	for _, mode := range []string{"ok", "foreign", "attached", "owner-reuse", "other-pool", "source-only", "lost-reply"} {
		t.Run(mode, func(t *testing.T) {
			p := snapshotVolumeFixture("oci")
			posts := 0
			v := persistentVolumeObservation{Name: p.target(), Type: "custom", ContentType: "filesystem", Config: p.targetConfig()}
			v.Config["volatile.idmap.last"] = "[]"
			if mode == "foreign" {
				v.Config["user.hacocoon.owner"] = "foreign"
			}
			if mode == "attached" {
				v.UsedBy = []string{"foreign"}
			}
			r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] != "query" {
					t.Fatal("non-copy command", args)
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
					if req.Name != "haco-persistent-"+strings.Repeat("d", 32) || req.Source["name"] != p.target() || req.Config["user.hacocoon.resource"] != "oci:restored" || req.Config["user.hacocoon.kind"] != OCIStoreKind || req.Config["user.hacocoon.snapshot-role"] != "" {
						t.Fatal(req)
					}
					if mode == "lost-reply" {
						return host.Result{ExitCode: 1}, core.ErrRuntimeUnavailable
					}
					return host.Result{}, nil
				}
				var value any
				if args[1] == "/1.0/storage-pools/pool" {
					value = map[string]string{"name": "pool", "driver": "btrfs"}
				} else if strings.Contains(args[1], "/volumes/custom?") {
					value = []persistentVolumeObservation{v}
				} else {
					t.Fatal("unexpected original/default dependency", args)
				}
				raw, _ := json.Marshal(value)
				return host.Result{Stdout: string(raw)}, nil
			}})
			c, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: r.project, Volume: &p})
			if err != nil {
				t.Fatal(err)
			}
			c.State = "verified"
			saved := core.Snapshot{ID: "snap-" + strings.Repeat("e", 32), State: "ready", Components: []core.SnapshotComponent{c}}
			b := &PersistentResourceBackend{Runtime: r}
			owner := strings.Repeat("d", 32)
			if mode == "owner-reuse" {
				owner = p.Owner
			}
			kind, native, err := b.PlanSavedResource(context.Background(), saved, owner)
			if mode == "foreign" || mode == "attached" || mode == "owner-reuse" {
				if err == nil || posts != 0 {
					t.Fatal("invalid plan", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			target := core.PersistentResource{ID: "oci:restored", Owner: owner, Kind: kind, NativeRef: native, State: "creating", RestoreSource: saved.ID}
			if mode == "other-pool" {
				target.NativeRef = "other/haco-persistent-" + owner
			}
			if mode == "source-only" {
				target.SourceOnly = true
			}
			err = b.CreateSavedResource(context.Background(), saved, target)
			if mode == "ok" {
				if err != nil || posts != 1 {
					t.Fatal(err, posts)
				}
			} else {
				if err == nil {
					t.Fatal("invalid copy accepted")
				}
				if mode != "lost-reply" && posts != 0 {
					t.Fatal("unexpected create")
				}
			}
		})
	}
}
