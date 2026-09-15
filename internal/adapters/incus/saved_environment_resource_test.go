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

func savedDataFixture(t *testing.T, r *Runtime) (core.Snapshot, snapshotVolumePlan, core.PersistentResource) {
	t.Helper()
	p := snapshotVolumeFixture("work")
	p.SourceKind, p.SourceID = CacheResourceKind, "env-data:"+strings.Repeat("1", 32)
	p.Source = "haco-persistent-" + p.SourceOwner
	p.Role, p.Device, p.Path = "data:build", environmentDataDevicePrefix+"build", "/var/cache/build"
	c, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: r.project, Volume: &p})
	if err != nil {
		t.Fatal(err)
	}
	c.State = "verified"
	saved := core.Snapshot{ID: "snap-" + strings.Repeat("c", 32), State: "ready", Components: []core.SnapshotComponent{c}, Source: core.SnapshotSource{
		InstanceID:  p.SourceInstanceID,
		Environment: core.Environment{Attachments: []core.EnvironmentAttachment{{Key: "build", Target: p.Path, Resource: core.PersistentResourceRef{ID: p.SourceID, Owner: p.SourceOwner}, Origin: core.ResourceGeneration{Kind: CacheResourceKind}}}},
	}}
	target := core.PersistentResource{ID: "env-data:" + strings.Repeat("2", 32), Owner: strings.Repeat("d", 32), Kind: CacheResourceKind, EnvironmentInstance: "env-" + strings.Repeat("e", 32), State: "creating", RestoreSource: saved.ID}
	target.NativeRef = "pool/haco-persistent-" + target.Owner
	return saved, p, target
}

func TestSavedEnvironmentDataRequiresMatchingVerifiedManifest(t *testing.T) {
	for _, failure := range []string{"incomplete", "missing-area", "missing-component", "duplicate-component", "unverified", "malformed-binding", "wrong-provider", "changed-kind", "changed-resource", "changed-owner", "changed-path", "changed-instance"} {
		t.Run(failure, func(t *testing.T) {
			b := &PersistentResourceBackend{Runtime: New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
				t.Error("invalid saved ownership reached provider")
				return host.Result{}, core.ErrRuntimeUnavailable
			}})}
			saved, _, target := savedDataFixture(t, b.Runtime)
			switch failure {
			case "incomplete":
				saved.State = "creating"
			case "missing-area":
				saved.Source.Environment.Attachments[0].Key = "other"
			case "missing-component":
				saved.Components = nil
			case "duplicate-component":
				saved.Components = append(saved.Components, saved.Components[0])
			case "unverified":
				saved.Components[0].State = "created"
			case "malformed-binding":
				saved.Components[0].Binding = "{}"
			case "wrong-provider":
				saved.Components[0].NativeRef = "other:volume/pool/haco-snapshot-" + saved.Components[0].Owner
			case "changed-kind":
				saved.Source.Environment.Attachments[0].Origin.Kind = OCIStoreKind
			case "changed-resource":
				saved.Source.Environment.Attachments[0].Resource.ID = "env-data:" + strings.Repeat("3", 32)
			case "changed-owner":
				saved.Source.Environment.Attachments[0].Resource.Owner = strings.Repeat("f", 32)
			case "changed-path":
				saved.Source.Environment.Attachments[0].Target = "/var/cache/other"
			case "changed-instance":
				saved.Source.InstanceID = "env-" + strings.Repeat("f", 32)
			}
			if kind, ref, err := b.PlanSavedEnvironmentResource(context.Background(), saved, "build", target.Owner); err == nil || kind != "" || ref != "" {
				t.Fatal("invalid manifest produced a destination", kind, ref, err)
			}
			if err := b.CreateSavedEnvironmentResource(context.Background(), saved, "build", target); err == nil {
				t.Fatal("create bypassed saved ownership validation")
			}
		})
	}
}

func TestSavedEnvironmentDataRequiresFreshDestinationReservation(t *testing.T) {
	b := &PersistentResourceBackend{Runtime: New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Error("invalid destination reached provider")
		return host.Result{}, core.ErrRuntimeUnavailable
	}})}
	saved, p, target := savedDataFixture(t, b.Runtime)
	for _, owner := range []string{"", "not-an-owner", p.Owner, p.SourceOwner} {
		if kind, ref, err := b.PlanSavedEnvironmentResource(context.Background(), saved, "build", owner); !errors.Is(err, core.ErrInvalidArgument) || kind != "" || ref != "" {
			t.Fatal("non-fresh owner accepted", kind, ref, err)
		}
	}
	for _, failure := range []string{"wrong-kind", "invalid-native-ref", "missing-instance", "same-instance", "source-only", "already-ready", "wrong-snapshot", "copy-source", "copy-completed", "saved-owner", "original-owner", "other-pool"} {
		t.Run(failure, func(t *testing.T) {
			changed := target
			switch failure {
			case "wrong-kind":
				changed.Kind = OCIStoreKind
			case "invalid-native-ref":
				changed.NativeRef = "../unrelated"
			case "missing-instance":
				changed.EnvironmentInstance = ""
			case "same-instance":
				changed.EnvironmentInstance = saved.Source.InstanceID
			case "source-only":
				changed.SourceOnly = true
			case "already-ready":
				changed.State = "ready"
			case "wrong-snapshot":
				changed.RestoreSource = "snap-" + strings.Repeat("f", 32)
			case "copy-source":
				changed.CopySource = saved.Source.Environment.Attachments[0].Resource
			case "copy-completed":
				changed.CopyCompleted = true
			case "saved-owner":
				changed.Owner = p.Owner
				changed.NativeRef = "pool/haco-persistent-" + changed.Owner
			case "original-owner":
				changed.Owner = p.SourceOwner
				changed.NativeRef = "pool/haco-persistent-" + changed.Owner
			case "other-pool":
				changed.NativeRef = "other/haco-persistent-" + changed.Owner
			}
			if err := b.CreateSavedEnvironmentResource(context.Background(), saved, "build", changed); err == nil {
				t.Fatal("stale or unrelated destination accepted")
			}
		})
	}
}

func TestSavedEnvironmentDataCopyRechecksSourceAndClearsOldAuthority(t *testing.T) {
	for _, failure := range []string{"none", "missing", "foreign", "busy", "lost-reply"} {
		t.Run(failure, func(t *testing.T) {
			var saved core.Snapshot
			var p snapshotVolumePlan
			var target core.PersistentResource
			var source persistentVolumeObservation
			copyStarted := false
			planned := false
			r := New(&fakeRunner{run: func(_ context.Context, _ int, command string, args []string) (host.Result, error) {
				if command != "incus" || args[0] != "query" {
					t.Fatal("unexpected native operation", command, args)
				}
				if args[1] == "-X" {
					copyStarted = true
					var request struct {
						Name, Type  string
						ContentType string `json:"content_type"`
						Config      map[string]string
						Source      map[string]any
					}
					if err := json.Unmarshal([]byte(args[len(args)-1]), &request); err != nil {
						t.Fatal(err)
					}
					if request.Name != "haco-persistent-"+target.Owner || request.Type != "custom" || request.ContentType != "filesystem" || request.Config["user.hacocoon.owner"] != target.Owner || request.Config["user.hacocoon.resource"] != target.ID || request.Config["user.hacocoon.kind"] != CacheResourceKind || request.Config[environmentInstanceKey] != target.EnvironmentInstance || request.Config["user.hacocoon.source-only"] != "false" {
						t.Fatal("copy omitted fresh destination ownership", request)
					}
					if !reflect.DeepEqual(request.Source, map[string]any{"type": "copy", "name": p.target(), "pool": p.Pool, "project": "hacocoon", "volume_only": true}) {
						t.Fatal("copy used a live source or another pool", request.Source)
					}
					for key := range source.Config {
						if key == "user.hacocoon.owner" || key == "user.hacocoon.kind" || key == environmentInstanceKey || key == "volatile.idmap.last" {
							continue
						}
						if value, exists := request.Config[key]; !exists || value != "" {
							t.Fatal("source authority was not explicitly cleared", key)
						}
					}
					if request.Config["volatile.idmap.last"] != "[]" {
						t.Fatal("Incus filesystem idmap bookkeeping lost")
					}
					if failure == "lost-reply" {
						return host.Result{}, errors.New("copy reply lost")
					}
					return host.Result{}, nil
				}
				if copyStarted {
					t.Fatal("fallible observation before caller could record completion")
				}
				if args[1] == "/1.0/storage-pools/pool" {
					return host.Result{Stdout: `{"name":"pool","driver":"btrfs"}`}, nil
				}
				if args[1] != "/1.0/storage-pools/pool/volumes/custom?project=hacocoon&recursion=1" {
					t.Fatal("restore depended on live Environment/Base/default profile", args)
				}
				volumes := []persistentVolumeObservation{source}
				if planned && failure == "missing" {
					volumes = []persistentVolumeObservation{}
				}
				raw, err := json.Marshal(volumes)
				return host.Result{Stdout: string(raw)}, err
			}})
			saved, p, target = savedDataFixture(t, r)
			source = persistentVolumeObservation{Name: p.target(), Type: "custom", ContentType: "filesystem", Config: p.targetConfig()}
			source.Config["volatile.idmap.last"] = "[]"
			source.Config["security.unmapped"] = "true"
			source.Config[environmentInstanceKey] = p.SourceInstanceID
			b := &PersistentResourceBackend{Runtime: r}
			kind, ref, err := b.PlanSavedEnvironmentResource(context.Background(), saved, "build", target.Owner)
			if err != nil || kind != target.Kind || ref != target.NativeRef {
				t.Fatal("owned saved data failed planning", kind, ref, err)
			}
			planned = true
			if failure == "foreign" {
				source.Config["user.hacocoon.owner"] = strings.Repeat("f", 32)
			}
			if failure == "busy" {
				source.UsedBy = []string{"/1.0/instances/other"}
			}
			err = b.CreateSavedEnvironmentResource(context.Background(), saved, "build", target)
			switch failure {
			case "none":
				if err != nil || !copyStarted {
					t.Fatal("copy failed", err)
				}
			case "lost-reply":
				if !errors.Is(err, core.ErrRecoveryRequired) || !copyStarted {
					t.Fatal("ambiguous copy lost recovery ownership", err)
				}
			default:
				if err == nil || copyStarted {
					t.Fatal("changed source copied after planning", err)
				}
				if kind, ref, err := b.PlanSavedEnvironmentResource(context.Background(), saved, "build", target.Owner); err == nil || kind != "" || ref != "" {
					t.Fatal("changed source produced a new plan", kind, ref, err)
				}
			}
		})
	}
}
