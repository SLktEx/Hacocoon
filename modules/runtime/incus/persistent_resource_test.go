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
	resource := core.PersistentResource{ID: "oci:demo", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32)}
	for _, scenario := range []string{"absent", "owned", "foreign", "busy", "malformed", "truncated", "still-present", "failed-delete-but-absent"} {
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
			if (scenario == "foreign" || scenario == "busy" || scenario == "malformed" || scenario == "truncated" || scenario == "absent") && deletes != 0 {
				t.Fatal("unsafe provider delete")
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
