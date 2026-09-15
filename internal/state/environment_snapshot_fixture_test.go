package state

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func savedDataFixture(t *testing.T) (*EnvironmentJSONStore, core.Snapshot) {
	t.Helper()
	ctx := context.Background()
	store, lease, plans := environmentDataFixture(t, 2)
	lease.SourcePath = "managed:source"
	lease.WorkspaceID = "source-work"
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(store.BeginEnvironmentCreateWithResources(ctx, lease, plans))
	for _, plan := range plans {
		r, err := store.BeginEnvironmentResourceMaterialization(ctx, lease, plan.Resource.Ref())
		must(err)
		r, err = store.RecordEnvironmentResourceCreated(ctx, r)
		must(err)
		must(store.CommitPersistentResourceCreate(ctx, r))
	}
	lease.RuntimeRef = "haco-source"
	must(store.RecordEnvironmentRuntime(ctx, lease))
	lease.State = core.WorkspaceLeaseActive
	env := core.Environment{Name: lease.EnvironmentID, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, RuntimeRef: lease.RuntimeRef, AccessMode: lease.AccessMode, Attachments: lease.Attachments, CreatedAt: lease.AcquiredAt}
	must(store.CommitEnvironmentCreate(ctx, env, lease))
	saved := core.Snapshot{ID: "snap-" + strings.Repeat("7", 32), State: "capturing", Source: core.SnapshotSource{Environment: env, InstanceID: lease.InstanceID}}
	for i, role := range []string{"rootfs", "workspace:main", "data:cache-00", "data:cache-01"} {
		saved.Components = append(saved.Components, core.SnapshotComponent{Role: role, NativeRef: "saved/" + role, Owner: fmt.Sprintf("%032x", 100+i), State: "planned"})
	}
	must(store.BeginSnapshot(ctx, saved))
	for _, c := range saved.Components {
		must(store.RecordSnapshotComponent(ctx, saved.ID, c, "created"))
		c.State = "created"
		must(store.RecordSnapshotComponent(ctx, saved.ID, c, "verified"))
	}
	must(store.CommitSnapshot(ctx, saved.ID))
	saved, err := store.GetSnapshot(ctx, saved.ID)
	must(err)
	return store, saved
}
