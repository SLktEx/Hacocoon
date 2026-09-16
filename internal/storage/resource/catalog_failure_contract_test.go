package persistentresource_test

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	persistentresource "github.com/SLktEx/Hacocoon/internal/storage/resource"
)

type checkedMaintenanceBackend struct{ *maintenanceBackend }

func (checkedMaintenanceBackend) CheckDeletion(context.Context, core.PersistentResource) error {
	return nil
}

func TestUnreadableCatalogCannotTriggerNativeMutationOrForgetOwnership(t *testing.T) {
	ctx := context.Background()
	svc, base, lease := activeResourceEnvironment(t)
	backend := checkedMaintenanceBackend{&maintenanceBackend{environmentContractBackend: base}}
	svc.Backend = backend
	area := lease.Attachments[0]
	publication, err := svc.PublishEnvironmentGeneration(ctx, lease, area)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(base.path)
	if err != nil {
		t.Fatal(err)
	}
	// A truncated catalog is not evidence that previously owned resources are
	// absent. Exercise the service through its actual file-backed adapter.
	corrupt := []byte(`{"version":10,"persistent_resources":`)
	if err := os.WriteFile(base.path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	importer := &importedDataBackend{&savedDataBackend{t: t, store: base.store}}
	importService := persistentresource.Service{Store: base.store, Backend: importer}
	request := core.EnvironmentResourceRequest{EnvironmentID: "imported", InstanceID: "env-" + strings.Repeat("c", 32), Workspace: core.Workspace{ID: "imported-work", Path: "managed:imported"}}
	inputs := []core.EnvironmentResourceImport{{Key: "compiler", Target: "/root/.cache/compiler", Kind: "build-cache", Digest: strings.Repeat("a", 64), Archive: strings.NewReader("saved bytes")}}
	ops := map[string]func() error{
		"create":       func() error { _, err := svc.Create(ctx, "oci:new", "oci-containerd"); return err },
		"copy":         func() error { _, err := svc.Copy(ctx, "oci:copy", area.Origin.Kind, area.Resource.ID); return err },
		"recover-copy": func() error { _, err := svc.RecoverCopy(ctx, publication.Candidate.Ref()); return err },
		"publish-generation": func() error {
			_, err := svc.PublishGeneration(ctx, publication.Generation, preparedGeneration)
			return err
		},
		"publish-environment-generation": func() error { _, err := svc.PublishEnvironmentGeneration(ctx, lease, area); return err },
		"recover-environment-generation": func() error { _, err := svc.RecoverEnvironmentGeneration(ctx, publication.Candidate.Ref()); return err },
		"delete-reviewed":                func() error { return svc.DeleteReviewed(ctx, area.Resource) },
		"delete-environment":             func() error { return svc.DeleteEnvironmentResources(ctx, lease) },
		"materialize-environment":        func() error { _, err := svc.MaterializeEnvironmentResources(ctx, lease); return err },
		"plan-import": func() error {
			_, err := importService.PlanImportedEnvironmentResources(ctx, request, inputs)
			return err
		},
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			if err := op(); err == nil {
				t.Fatal("unreadable ownership catalog was accepted")
			}
			after, err := os.ReadFile(base.path)
			if err != nil || !bytes.Equal(after, corrupt) || len(base.resources) != 3 || base.deletes != 0 || backend.copies != 1 || importer.copies != 0 {
				t.Fatal("catalog failure mutated native data or overwrote ownership", err)
			}
		})
	}
	if err := os.WriteFile(base.path, valid, 0600); err != nil {
		t.Fatal(err)
	}
	// Returning the original catalog makes the exact published owner observable;
	// none of the refusals requires repeating the copy or inventing a new owner.
	recovered, err := svc.RecoverEnvironmentGeneration(ctx, publication.Candidate.Ref())
	if err != nil || recovered.State != "published" || recovered.Candidate != publication.Candidate || backend.copies != 1 {
		t.Fatal("catalog recovery changed the published resource", recovered, err)
	}
}
