package persistentresource_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	persistentresource "github.com/SLktEx/Hacocoon/internal/storage/resource"
)

func TestOptionalCatalogContractsRefuseWithoutChangingOwnedData(t *testing.T) {
	svc, b, lease := activeResourceEnvironment(t)
	ctx := context.Background()
	area := lease.Attachments[0]
	resource, err := b.store.GetPersistentResource(ctx, area.Resource.ID)
	if err != nil {
		t.Fatal(err)
	}
	request := core.EnvironmentResourceRequest{EnvironmentID: lease.EnvironmentID, InstanceID: lease.InstanceID, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}}
	selection := []core.EnvironmentResourceSelection{{Key: area.Key, Target: area.Target, Origin: area.Origin}}
	inputs := []core.EnvironmentResourceImport{{Key: area.Key, Target: area.Target, Kind: area.Origin.Kind, Digest: area.Origin.Compatibility, Archive: strings.NewReader("saved bytes")}}
	before, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	// These operations need stronger atomic contracts than the basic catalog.
	// Hiding optional methods models a replaceable adapter with that limitation.
	svc.Store = struct{ persistentresource.Store }{b.store}
	ops := map[string]func() error{
		"plan-environment":               func() error { _, err := svc.PlanEnvironmentResources(ctx, request, selection); return err },
		"materialize-environment":        func() error { _, err := svc.MaterializeEnvironmentResources(ctx, lease); return err },
		"delete-environment":             func() error { return svc.DeleteEnvironmentResources(ctx, lease) },
		"clear-environment":              func() error { return svc.EmptyEnvironmentResource(ctx, lease, area) },
		"publish-environment-generation": func() error { _, err := svc.PublishEnvironmentGeneration(ctx, lease, area); return err },
		"publish-generation":             func() error { _, err := svc.PublishGeneration(ctx, area.Origin, preparedGeneration); return err },
		"recover-generation":             func() error { _, err := svc.RecoverEnvironmentGeneration(ctx, area.Resource); return err },
		"delete-generation":              func() error { return svc.DeleteUnselectedGeneration(ctx, area.Resource) },
		"plan-import":                    func() error { _, err := svc.PlanImportedEnvironmentResources(ctx, request, inputs); return err },
		"import-environment":             func() error { _, err := svc.ImportEnvironmentResources(ctx, lease, inputs); return err },
		"delete-reviewed":                func() error { return svc.DeleteReviewed(ctx, resource.Ref()) },
		"delete-restored":                func() error { return svc.DeleteRestoredCopy(ctx, resource) },
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			if err := op(); !errors.Is(err, core.ErrUnsupported) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(b.path)
			if err != nil || !bytes.Equal(before, after) || len(b.resources) != 2 || b.deletes != 0 {
				t.Fatal("unsupported operation changed the catalog or native data", err)
			}
		})
	}
}

func TestOptionalNativeFeaturesRefuseBeforeAllocatingData(t *testing.T) {
	svc, b, request, selections := environmentContractService(t)
	ctx := context.Background()
	publication, err := svc.PublishGeneration(ctx, selections[0].Origin, preparedGeneration)
	if err != nil {
		t.Fatal(err)
	}
	selections[0].Origin = publication.Generation
	lease, _ := reserveEnvironmentContract(t, svc, b.store, request, selections)
	inputs := []core.EnvironmentResourceImport{{Key: "compiler", Target: "/root/.cache/compiler", Kind: "build-cache", Digest: strings.Repeat("a", 64), Archive: strings.NewReader("saved bytes")}}
	before, err := os.ReadFile(b.path)
	if err != nil {
		t.Fatal(err)
	}
	plans := b.planAttempts
	svc.Backend = struct{ persistentresource.Backend }{b}
	ops := map[string]func() error{
		"copy": func() error {
			_, err := svc.Copy(ctx, "oci:copy", publication.Candidate.Kind, publication.Candidate.ID)
			return err
		},
		"plan-copy-environment": func() error { _, err := svc.PlanEnvironmentResources(ctx, request, selections); return err },
		"clear-environment":     func() error { return svc.EmptyEnvironmentResource(ctx, lease, lease.Attachments[0]) },
		"import": func() error {
			_, err := svc.Import(ctx, "oci:import", "oci-containerd", strings.NewReader("saved bytes"))
			return err
		},
		"plan-import":            func() error { _, err := svc.PlanImportedEnvironmentResources(ctx, request, inputs); return err },
		"import-environment":     func() error { _, err := svc.ImportEnvironmentResources(ctx, lease, inputs); return err },
		"plan-saved-environment": func() error { _, err := svc.PlanSavedEnvironmentResources(ctx, request, core.Snapshot{}); return err },
		"restore-snapshot":       func() error { _, err := svc.RestoreSnapshot(ctx, "oci:restored", core.Snapshot{}, "work"); return err },
		"delete-reviewed":        func() error { return svc.DeleteReviewed(ctx, publication.Candidate.Ref()) },
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			if err := op(); !errors.Is(err, core.ErrUnsupported) {
				t.Fatal(err)
			}
			after, err := os.ReadFile(b.path)
			if err != nil || !bytes.Equal(before, after) || len(b.resources) != 1 || b.deletes != 0 || b.planAttempts != plans {
				t.Fatal("unsupported native feature planned or changed owned data", err)
			}
		})
	}
}
