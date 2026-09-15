package incus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestPersistentVolumeDeletionRequiresExactOwnershipAndConfirmedAbsence(t *testing.T) {
	for _, kind := range []string{OCIStoreKind, CacheResourceKind} {
		t.Run(kind, func(t *testing.T) {
			resource := core.PersistentResource{ID: "oci:demo", Kind: kind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32)}
			for _, scenario := range []string{"absent", "owned", "foreign", "busy", "malformed", "truncated", "still-present", "failed-delete-but-absent", "snapshots", "backups", "schedule", "child-error", "query-exit"} {
				t.Run(scenario, func(t *testing.T) {
					removed := false
					deletes := 0
					runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
						if args[0] == "storage" {
							deletes++
							removed = true
							if scenario == "failed-delete-but-absent" {
								return host.Result{}, errors.New("lost reply")
							}
							return host.Result{}, nil
						}
						if strings.Contains(args[1], "/snapshots?") || strings.Contains(args[1], "/backups?") {
							if scenario == "child-error" {
								return host.Result{ExitCode: 1}, nil
							}
							if (scenario == "snapshots" && strings.Contains(args[1], "/snapshots?")) || (scenario == "backups" && strings.Contains(args[1], "/backups?")) {
								return host.Result{Stdout: `["saved"]`}, nil
							}
							return host.Result{Stdout: "[]"}, nil
						}
						if scenario == "query-exit" {
							return host.Result{ExitCode: 1, Stdout: "[]"}, nil
						}
						if scenario == "malformed" {
							return host.Result{Stdout: "{}"}, nil
						}
						if scenario == "truncated" {
							return host.Result{Stdout: "[]", StdoutTruncated: true}, nil
						}
						if scenario == "absent" || (removed && scenario != "still-present") {
							return host.Result{Stdout: "[]"}, nil
						}
						v := persistentVolumeObservation{Name: "haco-persistent-" + resource.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": resource.Owner, "user.hacocoon.resource": resource.ID, "user.hacocoon.kind": resource.Kind}}
						if scenario == "schedule" {
							v.Config["snapshots.schedule"] = "@daily"
						}
						if scenario == "foreign" {
							v.Config["user.hacocoon.owner"] = strings.Repeat("b", 32)
						}
						if scenario == "busy" {
							v.UsedBy = []string{"/1.0/instances/other"}
						}
						data, _ := json.Marshal([]persistentVolumeObservation{v})
						return host.Result{Stdout: string(data)}, nil
					}}
					err := (&PersistentResourceBackend{Runtime: New(runner)}).Delete(context.Background(), resource)
					success := scenario == "absent" || scenario == "owned" || scenario == "failed-delete-but-absent"
					if (err == nil) != success {
						t.Fatalf("err=%v", err)
					}
					if (scenario == "foreign" || scenario == "busy" || scenario == "malformed" || scenario == "truncated" || scenario == "absent" || scenario == "snapshots" || scenario == "backups" || scenario == "schedule" || scenario == "child-error" || scenario == "query-exit") && deletes != 0 {
						t.Fatal("unsafe provider delete")
					}
				})
			}
		})
	}
}

func TestPersistentVolumeAttachRefusesProviderUseOutsideCatalog(t *testing.T) {
	resource := core.PersistentResource{ID: "oci:demo", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32)}
	observation := persistentVolumeObservation{Name: "haco-persistent-" + resource.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": resource.Owner, "user.hacocoon.resource": resource.ID, "user.hacocoon.kind": resource.Kind}, UsedBy: []string{"/1.0/instances/foreign"}}
	data, _ := json.Marshal([]persistentVolumeObservation{observation})
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if args[0] != "query" {
			t.Fatalf("mutated a resource already in use: %v", args)
		}
		return host.Result{Stdout: string(data)}, nil
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.attachPersistentResource(context.Background(), "haco-dev", resource); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatalf("external RW attachment accepted: %v", err)
	}
}

func TestSourceOnlyResourcesCannotBeAttachedOrMasqueradeAsGuestStores(t *testing.T) {
	r := core.PersistentResource{ID: "oci-source:host", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), SourceOnly: true}
	p, _ := NewSandboxProvider(New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("source attachment reached provider mutation")
		return host.Result{}, nil
	}}))
	if err := p.attachPersistentResource(context.Background(), "haco-dev", r); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal(err)
	}
	for _, marker := range []string{"true", "false", "", "unexpected"} {
		if matchesSourceOnlyMarker(marker, true) != (marker == "true") {
			t.Fatal("source role confused")
		}
		if matchesSourceOnlyMarker(marker, false) != (marker == "false" || marker == "") {
			t.Fatal("guest role confused")
		}
	}
}

func TestCacheVolumesCannotEnterOCIOrTrustedHostPaths(t *testing.T) {
	ctx := context.Background()
	r := core.PersistentResource{ID: "oci-source:host", Kind: CacheResourceKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), SourceOnly: true, State: "creating"}
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("cache reached OCI/Host operation")
		return host.Result{}, nil
	}}
	b := &PersistentResourceBackend{Runtime: New(runner)}
	if _, _, err := persistentVolume(r); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if err := b.PrepareHostSource(ctx, r); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if err := b.VerifyHostSource(ctx, r); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	if b.hostCopyConsumer(r, &persistentVolumeObservation{UsedBy: []string{"/1.0/instances/haco-host"}}) {
		t.Fatal("cache accepted as Host data")
	}
	p, err := NewSandboxProvider(b.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	r.SourceOnly = false
	if err := p.attachPersistentResource(ctx, "haco-dev", r); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	target := r
	target.Kind = OCIStoreKind
	target.ID = "oci:target"
	target.Owner = strings.Repeat("b", 32)
	target.NativeRef = "pool/haco-persistent-" + target.Owner
	target.CopySource = r.Ref()
	if err := b.Copy(ctx, r, target); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}

func TestAttachedCacheSourceIsNeverQuiescedAsHost(t *testing.T) {
	ctx := context.Background()
	source := core.PersistentResource{ID: "generation:" + strings.Repeat("a", 32), Kind: CacheResourceKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), SourceOnly: true}
	target := core.PersistentResource{ID: "cache:copy", Kind: CacheResourceKind, Owner: strings.Repeat("b", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("b", 32), CopySource: source.Ref()}
	v := persistentVolumeObservation{Name: "haco-persistent-" + source.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": source.Owner, "user.hacocoon.resource": source.ID, "user.hacocoon.kind": source.Kind, "user.hacocoon.source-only": "true"}, UsedBy: []string{"/1.0/instances/haco-host"}}
	data, err := json.Marshal([]persistentVolumeObservation{v})
	if err != nil {
		t.Fatal(err)
	}
	b := &PersistentResourceBackend{Runtime: New(&fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" || args[0] != "query" {
			t.Fatalf("unexpected command %s %v", name, args)
		}
		switch args[1] {
		case "/1.0/storage-pools/pool":
			return host.Result{Stdout: `{"name":"pool","driver":"btrfs"}`}, nil
		case "/1.0/storage-pools/pool/volumes/custom?project=hacocoon&recursion=1":
			return host.Result{Stdout: string(data)}, nil
		default:
			t.Fatalf("cache inspected or changed Host: %v", args)
			return host.Result{}, nil
		}
	}})}
	if err := b.Verify(ctx, source); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal(err)
	}
	if err := b.Copy(ctx, source, target); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal(err)
	}
}
