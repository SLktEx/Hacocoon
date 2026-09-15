package incus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func snapshotVolumeFixture(kind string) snapshotVolumePlan {
	p := snapshotVolumePlan{Pool: "pool", Source: "haco-work-one", SourceOwner: strings.Repeat("a", 32), SourceKind: "work", SourceID: "repo", SourceInstance: "haco-env", SourceInstanceID: testEnvironmentInstance, Owner: strings.Repeat("b", 32), Role: "workspace:repo"}
	if kind == "oci" {
		p.SourceKind = OCIStoreKind
		p.SourceID = "oci:one"
		p.Source = "haco-persistent-" + p.SourceOwner
		p.Role = "oci"
	}
	return p
}
func snapshotSourceObservation(p snapshotVolumePlan) persistentVolumeObservation {
	c := map[string]string{"user.hacocoon.owner": p.SourceOwner, "user.hacocoon.role": "work", "user.hacocoon.repository": p.SourceID, "user.hacocoon.kind": p.SourceKind, "user.hacocoon.resource": p.SourceID, "user.hacocoon.source-only": "false", "security.unmapped": "true", "volatile.idmap.last": "[]"}
	return persistentVolumeObservation{Name: p.Source, Type: "custom", ContentType: "filesystem", Config: c, UsedBy: []string{"/1.0/instances/" + p.SourceInstance + "?project=hacocoon"}}
}
func TestSnapshotVolumeCopyBindsSourceAndIndependentTarget(t *testing.T) {
	for _, kind := range []string{"work", "oci"} {
		for _, mode := range []string{"ok", "running", "state-truncated", "instance-replaced", "foreign-owner", "busy", "host-source", "duplicate", "missing", "bad-idmap", "not-btrfs", "malformed", "lost-reply"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				p := snapshotVolumeFixture(kind)
				posts := 0
				runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
					if name != "incus" {
						t.Fatal(name)
					}
					switch args[0] {
					case "config":
						id := p.SourceInstanceID
						if mode == "instance-replaced" {
							id = "env-22222222222222222222222222222222"
						}
						return host.Result{Stdout: id}, nil
					case "list":
						status := "STOPPED"
						if mode == "running" {
							status = "RUNNING"
						}
						return host.Result{Stdout: p.SourceInstance + "," + status, StdoutTruncated: mode == "state-truncated"}, nil
					case "query":
						if args[1] == "/1.0/storage-pools/pool" {
							driver := "btrfs"
							if mode == "not-btrfs" {
								driver = "dir"
							}
							return host.Result{Stdout: `{"name":"pool","driver":"` + driver + `"}`}, nil
						}
						if args[1] == "-X" {
							posts++
							var request struct {
								Name   string
								Config map[string]string
								Source map[string]any
							}
							if err := json.Unmarshal([]byte(args[len(args)-1]), &request); err != nil {
								t.Fatal(err)
							}
							if request.Name != p.target() || request.Config["user.hacocoon.owner"] != p.Owner || request.Config["user.hacocoon.snapshot-source-owner"] != p.SourceOwner || request.Config["volatile.idmap.last"] != "[]" || request.Config["security.unmapped"] != "" {
								t.Fatal("unsafe copy config", request)
							}
							if !reflect.DeepEqual(request.Source, map[string]any{"type": "copy", "name": p.Source, "pool": p.Pool, "project": "hacocoon", "volume_only": true}) {
								t.Fatal(request.Source)
							}
							if mode == "lost-reply" {
								return host.Result{}, errors.New("connection lost")
							}
							return host.Result{}, nil
						}
						observed := snapshotSourceObservation(p)
						switch mode {
						case "foreign-owner":
							observed.Config["user.hacocoon.owner"] = "foreign"
						case "busy":
							observed.UsedBy = []string{"/1.0/instances/haco-other?project=hacocoon"}
						case "host-source":
							observed.UsedBy = []string{"/1.0/instances/haco-host?project=hacocoon"}
						case "bad-idmap":
							observed.Config["volatile.idmap.last"] = "null"
						case "malformed":
							return host.Result{Stdout: "{}"}, nil
						}
						list := []persistentVolumeObservation{observed}
						if mode == "duplicate" {
							list = append(list, observed)
						}
						if mode == "missing" {
							list = nil
							return host.Result{Stdout: "[]"}, nil
						}
						data, _ := json.Marshal(list)
						return host.Result{Stdout: string(data)}, nil
					}
					t.Fatal(args)
					return host.Result{}, nil
				}}
				err := New(runner).createSnapshotVolume(context.Background(), p)
				if mode == "ok" {
					if err != nil || posts != 1 {
						t.Fatal(err, posts)
					}
				} else if mode == "lost-reply" {
					if !errors.Is(err, core.ErrRecoveryRequired) || posts != 1 {
						t.Fatal(err, posts)
					}
				} else if err == nil || posts != 0 {
					t.Fatal("unsafe source copied", err, posts)
				}
			})
		}
	}
}
func TestSnapshotVolumeDeletionRequiresOwnedUnusedAndObservedAbsence(t *testing.T) {
	for _, mode := range []string{"ok", "absent", "foreign", "busy", "duplicate", "malformed", "delete-failed", "still-present", "observation-failed"} {
		t.Run(mode, func(t *testing.T) {
			p := snapshotVolumeFixture("work")
			deletes := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] == "storage" {
					deletes++
					if !reflect.DeepEqual(args, []string{"storage", "volume", "delete", p.Pool, p.target(), "--project", "hacocoon"}) {
						t.Fatal(args)
					}
					if mode == "delete-failed" {
						return host.Result{}, errors.New("failed")
					}
					return host.Result{}, nil
				}
				if mode == "observation-failed" && deletes > 0 {
					return host.Result{}, errors.New("unavailable")
				}
				if mode == "malformed" {
					return host.Result{Stdout: "null"}, nil
				}
				if mode == "absent" || (deletes > 0 && mode != "still-present") {
					return host.Result{Stdout: "[]"}, nil
				}
				v := persistentVolumeObservation{Name: p.target(), Type: "custom", ContentType: "filesystem", Config: p.targetConfig()}
				if mode == "foreign" {
					v.Config["user.hacocoon.owner"] = "foreign"
				}
				if mode == "busy" {
					v.UsedBy = []string{"/1.0/instances/haco-env?project=hacocoon"}
				}
				list := []persistentVolumeObservation{v}
				if mode == "duplicate" {
					list = append(list, v)
				}
				data, _ := json.Marshal(list)
				return host.Result{Stdout: string(data)}, nil
			}}
			err := New(runner).deleteSnapshotVolume(context.Background(), p)
			if mode == "ok" || mode == "absent" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("ambiguous cleanup released")
			}
			if (mode == "foreign" || mode == "busy" || mode == "duplicate" || mode == "malformed" || mode == "absent") && deletes != 0 {
				t.Fatal("unsafe delete", deletes)
			}
		})
	}
}
func TestSnapshotVolumeInvalidBindingsNeverReachProvider(t *testing.T) {
	for _, mode := range []string{"pool", "source", "target-owner", "same-owner", "host", "source-kind", "role", "source-id"} {
		t.Run(mode, func(t *testing.T) {
			p := snapshotVolumeFixture("work")
			switch mode {
			case "pool":
				p.Pool = "../foreign"
			case "source":
				p.Source = "--all"
			case "target-owner":
				p.Owner = "bad"
			case "same-owner":
				p.Owner = p.SourceOwner
			case "host":
				p.SourceInstance = "haco-host"
			case "source-kind":
				p.SourceKind = "repo"
			case "role":
				p.Role = "workspace:../bad"
			case "source-id":
				p.SourceID = "../bad"
			}
			runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
				t.Fatal("invalid binding reached provider")
				return host.Result{}, nil
			}}
			r := New(runner)
			if r.createSnapshotVolume(context.Background(), p) == nil || r.deleteSnapshotVolume(context.Background(), p) == nil {
				t.Fatal("invalid binding accepted")
			}
		})
	}
}
