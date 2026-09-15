package persistentresource_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
)

func activeResourceEnvironment(t *testing.T) (*persistentresource.Service, *environmentContractBackend, core.WorkspaceLease) {
	t.Helper()
	svc, backend, request, selections := environmentContractService(t)
	ctx := context.Background()
	lease, _ := reserveEnvironmentContract(t, svc, backend.store, request, selections)
	if _, err := svc.MaterializeEnvironmentResources(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.RuntimeRef = "haco-builder"
	if err := backend.store.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatal(err)
	}
	lease.State = core.WorkspaceLeaseActive
	env := core.Environment{Name: lease.EnvironmentID, Workspace: request.Workspace, RuntimeRef: lease.RuntimeRef, AccessMode: lease.AccessMode, Attachments: lease.Attachments, CreatedAt: lease.AcquiredAt}
	if err := backend.store.CommitEnvironmentCreate(ctx, env, lease); err != nil {
		t.Fatal(err)
	}
	return svc, backend, lease
}

type maintenanceBackend struct {
	*environmentContractBackend
	contents   map[string]string
	emptyErr   error
	emptyCalls int
	copies     int
}

func (b *maintenanceBackend) EmptyEnvironmentResource(ctx context.Context, lease core.WorkspaceLease, area core.EnvironmentAttachment, resource core.PersistentResource) error {
	b.emptyCalls++
	held, err := b.store.GetPersistentResource(ctx, resource.ID)
	if err != nil || held != resource || held.State != "clearing" || resource.EnvironmentInstance != lease.InstanceID || resource.Ref() != area.Resource {
		return errors.New("clear before exact durable ownership fence")
	}
	if err := b.Verify(ctx, resource); err != nil {
		return err
	}
	b.contents[resource.ID] = ""
	return b.emptyErr
}
func (b *maintenanceBackend) Copy(ctx context.Context, source, target core.PersistentResource) error {
	b.copies++
	return b.environmentContractBackend.Copy(ctx, source, target)
}

type maintenanceCommitFault struct {
	*state.EnvironmentJSONStore
	err   error
	after bool
}

func (f *maintenanceCommitFault) CommitEnvironmentResourceClear(ctx context.Context, lease core.WorkspaceLease, resource core.PersistentResource) error {
	if f.err == nil || f.after {
		if err := f.EnvironmentJSONStore.CommitEnvironmentResourceClear(ctx, lease, resource); err != nil {
			return err
		}
	}
	return f.err
}

func TestClearResourceKeepsOwnershipAndPersistsUncertainOutcome(t *testing.T) {
	for _, mode := range []string{"complete", "native-failed", "commit-before", "commit-after", "foreign-area", "foreign-lease"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			svc, base, lease := activeResourceEnvironment(t)
			backend := &maintenanceBackend{environmentContractBackend: base, contents: map[string]string{}}
			for _, area := range lease.Attachments {
				backend.contents[area.Resource.ID] = "retained bytes"
			}
			svc.Backend = backend
			failure := errors.New("maintenance outcome uncertain")
			fault := &maintenanceCommitFault{EnvironmentJSONStore: base.store}
			svc.Store = fault
			area := lease.Attachments[0]
			target := lease
			switch mode {
			case "native-failed":
				backend.emptyErr = failure
			case "commit-before":
				fault.err = failure
			case "commit-after":
				fault.err, fault.after = failure, true
			case "foreign-area":
				area.Target += "/unreviewed"
			case "foreign-lease":
				target.Owner = "other-owner"
			}
			err := svc.EmptyEnvironmentResource(ctx, target, area)
			invalid := mode == "foreign-area" || mode == "foreign-lease"
			if mode == "complete" {
				if err != nil {
					t.Fatal(err)
				}
			} else if invalid {
				if err == nil || backend.emptyCalls != 0 {
					t.Fatal("changed review reached native clear", err)
				}
			} else if !errors.Is(err, failure) || !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal("uncertain clear lost recovery requirement", err)
			}
			reopened := state.NewEnvironmentJSONStore(base.path)
			held, err := reopened.GetPersistentResource(ctx, lease.Attachments[0].Resource.ID)
			wantState := "ready"
			if mode == "native-failed" || mode == "commit-before" {
				wantState = "clearing"
			}
			if err != nil || held.Ref() != lease.Attachments[0].Resource || held.EnvironmentInstance != lease.InstanceID || held.State != wantState {
				t.Fatal("clear lost durable owner or fence", held, err)
			}
			if backend.contents[lease.Attachments[1].Resource.ID] != "retained bytes" || len(base.resources) != 2 || base.deletes != 0 {
				t.Fatal("clear deleted ownership or changed another area")
			}
			if wantState == "clearing" {
				if err := reopened.CheckEnvironmentResourcesIdle(ctx, lease.EnvironmentID); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("incomplete clear did not fence writers", err)
				}
				backend.emptyErr, fault.err = nil, nil
				if err := svc.EmptyEnvironmentResource(ctx, lease, lease.Attachments[0]); err != nil {
					t.Fatal(err)
				}
				if err := reopened.CheckEnvironmentResourcesIdle(ctx, lease.EnvironmentID); err != nil {
					t.Fatal("completed clear kept writers fenced", err)
				}
			}
		})
	}
}

type publicationCommitFault struct {
	*state.EnvironmentJSONStore
	err   error
	after bool
}

func (f *publicationCommitFault) AdvanceResourceGeneration(ctx context.Context, expected core.ResourceGeneration, ref core.PersistentResourceRef) (core.ResourceGeneration, error) {
	if f.err == nil || f.after {
		result, err := f.EnvironmentJSONStore.AdvanceResourceGeneration(ctx, expected, ref)
		if err != nil {
			return result, err
		}
		return result, f.err
	}
	return core.ResourceGeneration{}, f.err
}

func TestPublicationPersistenceRecoveryNeverRecopiesOrDeletesSelectedData(t *testing.T) {
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before-write", true: "after-write"}[after], func(t *testing.T) {
			ctx := context.Background()
			svc, base, lease := activeResourceEnvironment(t)
			backend := &maintenanceBackend{environmentContractBackend: base}
			svc.Backend = backend
			failure := errors.New("generation write outcome uncertain")
			fault := &publicationCommitFault{EnvironmentJSONStore: base.store, err: failure, after: after}
			svc.Store = fault
			area := lease.Attachments[0]
			publication, err := svc.PublishEnvironmentGeneration(ctx, lease, area)
			if !errors.Is(err, failure) || !errors.Is(err, core.ErrRecoveryRequired) || publication.State != "recovery-required" || publication.Candidate.State != "ready" || backend.copies != 1 || base.deletes != 0 {
				t.Fatal("uncertain publication lost candidate", publication, err)
			}
			reopened := state.NewEnvironmentJSONStore(base.path)
			svc.Store = reopened
			held, err := reopened.GetPersistentResource(ctx, publication.Candidate.ID)
			if err != nil || held != publication.Candidate || held.Producer != area.Resource || held.PublicationOrigin != area.Origin {
				t.Fatal("durable provenance changed", held, err)
			}
			recovered, err := svc.RecoverEnvironmentGeneration(ctx, held.Ref())
			if err != nil || recovered.State != "published" || recovered.Generation.Current != held.Ref() || backend.copies != 1 || base.deletes != 0 {
				t.Fatal("recovery repeated copy or lost selected data", recovered, err)
			}
			if err := svc.DeleteUnselectedGeneration(ctx, held.Ref()); !errors.Is(err, core.ErrStorageBusy) {
				t.Fatal("current generation was deleted", err)
			}
			again, err := svc.PublishEnvironmentGeneration(ctx, lease, area)
			if err != nil || again.State != "skipped" || backend.copies != 1 {
				t.Fatal("stale origin copied again", again, err)
			}
			if _, err := reopened.ResetResourceGeneration(ctx, recovered.Generation, recovered.Generation.Compatibility); err != nil {
				t.Fatal(err)
			}
			retained, err := svc.RecoverEnvironmentGeneration(ctx, held.Ref())
			if err != nil || retained.State != "retained" || backend.copies != 1 || base.deletes != 0 {
				t.Fatal("old selected data was deleted during recovery", retained, err)
			}
			if err := svc.DeleteUnselectedGeneration(ctx, held.Ref()); err != nil {
				t.Fatal(err)
			}
			if _, err := reopened.GetPersistentResource(ctx, held.ID); !errors.Is(err, core.ErrNotFound) || base.deletes != 1 || len(base.resources) != 2 {
				t.Fatal("explicit deletion lost source owners", err)
			}
		})
	}
}
