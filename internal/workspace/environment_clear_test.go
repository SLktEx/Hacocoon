package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (b *environmentDataBackend) EmptyEnvironmentResource(ctx context.Context, lease core.WorkspaceLease, area core.EnvironmentAttachment, r core.PersistentResource) error {
	held, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || held != r || held.State != "clearing" || lease.InstanceID != r.EnvironmentInstance || area.Resource != r.Ref() {
		return errors.New("empty without durable ownership fence")
	}
	if b.resources[r.ID].Ref() != r.Ref() {
		return core.ErrCapabilityStale
	}
	if b.fail == "empty" {
		return errors.New("partially emptied")
	}
	return nil
}

func TestEmptyEnvironmentCacheRetainsOwnershipAndOtherData(t *testing.T) {
	ctx := context.Background()
	svc, catalog, b, spec, _ := dataEnvironmentService(t, "")
	svc.runtime = &collectingRuntime{dataEnvironmentRuntime: svc.runtime.(*dataEnvironmentRuntime), status: core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}}
	env, err := svc.Create(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	published, err := svc.CollectEnvironmentResource(ctx, env.Name, "compiler")
	if err != nil {
		t.Fatal(err)
	}
	before, err := catalog.ListPersistentResources(ctx)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := catalog.GetWorkspaceLease(ctx, env.Name)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.EmptyEnvironmentResource(ctx, env.Name, env.Attachments[0]); err != nil {
		t.Fatal(err)
	}
	after, err := catalog.ListPersistentResources(ctx)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("changed retained ownership", before, after, err)
	}
	held, err := catalog.GetWorkspaceLease(ctx, env.Name)
	if err != nil || !held.Equal(lease) {
		t.Fatal("released lease", held, err)
	}
	actual, err := catalog.GetEnvironment(ctx, env.Name)
	if err != nil || !reflect.DeepEqual(actual, env) {
		t.Fatal("changed Env", actual, err)
	}
	if len(b.resources) != 4 || b.resources[published.Candidate.ID].Ref() != published.Candidate.Ref() {
		t.Fatal("removed common source or retained OCI")
	}
	if _, err := svc.provider.Resolve(ctx, WorkspaceRequest{Path: spec.WorkspacePath}); err != nil {
		t.Fatal("removed Workspace", err)
	}
}

func TestInterruptedCacheEmptyRemainsFencedUntilExplicitRetry(t *testing.T) {
	ctx := context.Background()
	svc, catalog, b, spec, resources := dataEnvironmentService(t, "")
	svc.runtime = &collectingRuntime{dataEnvironmentRuntime: svc.runtime.(*dataEnvironmentRuntime), status: core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}}
	env, err := svc.Create(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	b.fail = "empty"
	if err := svc.EmptyEnvironmentResource(ctx, env.Name, env.Attachments[0]); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	for _, op := range []func() error{
		func() error { return svc.Start(ctx, env.Name) }, func() error { return svc.Delete(ctx, env.Name) },
		func() error { return catalog.CheckEnvironmentResourcesIdle(ctx, env.Name) },
		func() error { return svc.EmptyEnvironmentResource(ctx, env.Name, env.Attachments[1]) },
		func() error { _, err := svc.CollectEnvironmentResource(ctx, env.Name, "compiler"); return err },
	} {
		if err := op(); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatal("lost persisted maintenance fence", err)
		}
	}
	r, err := catalog.GetPersistentResource(ctx, env.Attachments[0].Resource.ID)
	if err != nil || r.State != "clearing" {
		t.Fatal(r, err)
	}
	if err := resources.Delete(ctx, r.ID); err == nil {
		t.Fatal("generic cleanup adopted cache")
	}
	lease, err := catalog.GetWorkspaceLease(ctx, env.Name)
	if err != nil {
		t.Fatal(err)
	}
	stale := lease
	stale.RuntimeRef = "foreign"
	if err := catalog.CommitEnvironmentResourceClear(ctx, stale, r); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("wrong receipt cleared fence", err)
	}
	b.fail = ""
	if err := svc.EmptyEnvironmentResource(ctx, env.Name, env.Attachments[0]); err != nil {
		t.Fatal("retry", err)
	}
	if err := catalog.CheckEnvironmentResourcesIdle(ctx, env.Name); err != nil {
		t.Fatal("retry did not release maintenance fence", err)
	}
	if len(b.resources) != 3 {
		t.Fatal("retry deleted owned data", b.resources)
	}
}

func TestCacheEmptyRefusesRunningForeignOrChangedReview(t *testing.T) {
	for _, mode := range []string{"running", "absent", "foreign", "changed-area", "wrong-lease"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, catalog, _, spec, _ := dataEnvironmentService(t, "")
			runtime := &collectingRuntime{dataEnvironmentRuntime: svc.runtime.(*dataEnvironmentRuntime), status: core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}}
			svc.runtime = runtime
			env, err := svc.Create(ctx, spec)
			if err != nil {
				t.Fatal(err)
			}
			area := env.Attachments[0]
			switch mode {
			case "running":
				runtime.status.State = core.EnvironmentRunning
			case "absent":
				runtime.status.Absent = true
			case "foreign":
				runtime.identityErr = core.ErrCapabilityStale
			case "changed-area":
				area.Target = "/root/.cache/other"
			}
			if mode == "wrong-lease" {
				lease, err := catalog.GetWorkspaceLease(ctx, env.Name)
				if err != nil {
					t.Fatal(err)
				}
				lease.AccessMode = core.WorkspaceReadOnly
				_, err = catalog.BeginEnvironmentResourceClear(ctx, lease, area.Resource)
				if err == nil {
					t.Fatal("wrong lease accepted")
				}
			} else if err := svc.EmptyEnvironmentResource(ctx, env.Name, area); err == nil {
				t.Fatal("unsafe clear accepted")
			}
			r, err := catalog.GetPersistentResource(ctx, area.Resource.ID)
			if err != nil || r.State != "ready" {
				t.Fatal("rejected request changed state", r, err)
			}
		})
	}
}
