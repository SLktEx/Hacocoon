package workspace

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type collectingRuntime struct {
	*dataEnvironmentRuntime
	status      core.EnvironmentRuntimeStatus
	identityErr error
}

func (r *collectingRuntime) VerifyEnvironmentIdentity(context.Context, string, string) error {
	return r.identityErr
}
func (r *collectingRuntime) InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
	return r.status, nil
}

func TestOrdinaryEnvironmentPublishesWholeGenerationAndKeepsRetainedData(t *testing.T) {
	ctx := context.Background()
	svc, catalog, backend, spec, resources := dataEnvironmentService(t, "")
	runtime := &collectingRuntime{dataEnvironmentRuntime: svc.runtime.(*dataEnvironmentRuntime), status: core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}}
	svc.runtime = runtime
	env, err := svc.Create(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.CollectEnvironmentResource(ctx, env.Name, "compiler")
	if err != nil || result.State != "published" || result.Generation.Number != 1 || result.Generation.Current != result.Candidate.Ref() {
		t.Fatal(result, err)
	}
	if result.Candidate.EnvironmentInstance != "" || !result.Candidate.SourceOnly {
		t.Fatal("published source retained producer ownership")
	}
	again, err := svc.CollectEnvironmentResource(ctx, env.Name, "compiler")
	if err != nil || again.State != "skipped" || again.Candidate.ID != "" {
		t.Fatal(again, err)
	}
	// A new independent copy can consume the generation after producer deletion.
	if err := svc.Delete(ctx, env.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GetPersistentResource(ctx, env.PersistentResource.ID); err != nil {
		t.Fatal("lost retained OCI", err)
	}
	copy, err := resources.Copy(ctx, "cache:next", result.Candidate.Kind, result.Candidate.ID)
	if err != nil || copy.Owner == result.Candidate.Owner || backend.resources[copy.ID].Ref() != copy.Ref() {
		t.Fatal(copy, err)
	}
}

func TestUncertainCollectionFencesProducerAcrossReload(t *testing.T) {
	ctx := context.Background()
	svc, catalog, backend, spec, _ := dataEnvironmentService(t, "")
	svc.runtime = &collectingRuntime{dataEnvironmentRuntime: svc.runtime.(*dataEnvironmentRuntime), status: core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}}
	env, err := svc.Create(ctx, spec)
	if err != nil {
		t.Fatal(err)
	}
	backend.fail = "copy"
	result, err := svc.CollectEnvironmentResource(ctx, env.Name, "compiler")
	if !errors.Is(err, core.ErrRecoveryRequired) || result.State != "recovery-required" || result.Candidate.ID == "" {
		t.Fatal(result, err)
	}
	if err := svc.Start(ctx, env.Name); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("restart was not fenced", err)
	}
	if err := svc.Delete(ctx, env.Name); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("delete was not fenced", err)
	}
	lease, err := catalog.GetWorkspaceLease(ctx, env.Name)
	if err != nil || lease.RuntimeAbsent {
		t.Fatal(lease, err)
	}
	if _, err := catalog.PrepareEnvironmentResourceDeletion(ctx, lease); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("absence receipt released source", err)
	}
	// The catalog reads the persisted copy source fence for every operation.
	if err := catalog.CheckEnvironmentResourcesIdle(ctx, env.Name); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
}

func TestCollectionRefusesRunningForeignAndUnselectedSources(t *testing.T) {
	for _, mode := range []string{"running", "absent", "foreign", "unselected"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, catalog, _, spec, _ := dataEnvironmentService(t, "")
			runtime := &collectingRuntime{dataEnvironmentRuntime: svc.runtime.(*dataEnvironmentRuntime), status: core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}}
			svc.runtime = runtime
			env, err := svc.Create(ctx, spec)
			if err != nil {
				t.Fatal(err)
			}
			key := "compiler"
			switch mode {
			case "running":
				runtime.status.State = core.EnvironmentRunning
			case "absent":
				runtime.status.Absent = true
			case "foreign":
				runtime.identityErr = core.ErrCapabilityStale
			case "unselected":
				key = "other"
			}
			before, err := catalog.ListPersistentResources(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.CollectEnvironmentResource(ctx, env.Name, key); err == nil {
				t.Fatal("unsafe source accepted")
			}
			after, err := catalog.ListPersistentResources(ctx)
			if err != nil || len(before) != len(after) {
				t.Fatal("allocated rejected source", err)
			}
		})
	}
}
