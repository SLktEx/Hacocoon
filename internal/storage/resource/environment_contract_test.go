package persistentresource_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	persistentresource "github.com/SLktEx/Hacocoon/internal/storage/resource"
)

type environmentContractBackend struct {
	*generationBackend
	path         string
	planErr      error
	emptyPlan    bool
	planAttempts int
}

func (b *environmentContractBackend) Plan(ctx context.Context, kind, owner string) (string, error) {
	b.planAttempts++
	if b.planErr != nil {
		return "", b.planErr
	}
	if b.emptyPlan {
		return "", nil
	}
	return b.generationBackend.Plan(ctx, kind, owner)
}

// Only native copy mechanics are substituted; the real catalog reserves both
// exact owners and the service must record completion before publication.
func (b *environmentContractBackend) Copy(ctx context.Context, source, target core.PersistentResource) error {
	held, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || held != target || held.CopySource != source.Ref() {
		return errors.New("copy before durable ownership")
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.resources[source.ID].Ref() != source.Ref() {
		return core.ErrCapabilityStale
	}
	b.resources[target.ID] = target
	return nil
}

func environmentContractService(t *testing.T) (*persistentresource.Service, *environmentContractBackend, core.EnvironmentResourceRequest, []core.EnvironmentResourceSelection) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	store := state.NewEnvironmentJSONStore(path)
	b := &environmentContractBackend{path: path, generationBackend: &generationBackend{store: store, resources: map[string]core.PersistentResource{}}}
	selections := make([]core.EnvironmentResourceSelection, 0, 2)
	for _, key := range []string{"packages", "compiler"} {
		origin, err := store.EnsureResourceGeneration(context.Background(), key, "build-cache", strings.Repeat("a", 64))
		if err != nil {
			t.Fatal(err)
		}
		selections = append(selections, core.EnvironmentResourceSelection{Key: key, Target: "/root/.cache/" + key, Origin: origin})
	}
	request := core.EnvironmentResourceRequest{EnvironmentID: "builder", InstanceID: "env-" + strings.Repeat("b", 32), Workspace: core.Workspace{ID: "workspace", Path: "managed:workspace"}}
	return &persistentresource.Service{Store: store, Backend: b}, b, request, selections
}

func reserveEnvironmentContract(t *testing.T, svc *persistentresource.Service, catalog *state.EnvironmentJSONStore, request core.EnvironmentResourceRequest, selections []core.EnvironmentResourceSelection) (core.WorkspaceLease, []core.EnvironmentResourcePlan) {
	t.Helper()
	plans, err := svc.PlanEnvironmentResources(context.Background(), request, selections)
	if err != nil {
		t.Fatal(err)
	}
	lease := core.WorkspaceLease{EnvironmentID: request.EnvironmentID, Owner: request.EnvironmentID, InstanceID: request.InstanceID, WorkspaceID: request.Workspace.ID, SourcePath: request.Workspace.Path, AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC()}
	for _, plan := range plans {
		lease.Attachments = append(lease.Attachments, plan.Attachment)
	}
	if err := catalog.BeginEnvironmentCreateWithResources(context.Background(), lease, plans); err != nil {
		t.Fatal(err)
	}
	return lease, plans
}

func TestEnvironmentResourcePlanCanonicalizesWithoutAllocatingOrAdoptingData(t *testing.T) {
	svc, b, request, selections := environmentContractService(t)
	ctx := context.Background()
	before := append([]core.EnvironmentResourceSelection(nil), selections...)
	first, err := svc.PlanEnvironmentResources(ctx, request, selections)
	if err != nil || len(first) != 2 {
		t.Fatal(first, err)
	}
	second, err := svc.PlanEnvironmentResources(ctx, request, selections)
	if err != nil || len(second) != 2 {
		t.Fatal(second, err)
	}
	if !reflect.DeepEqual(selections, before) {
		t.Fatal("planning mutated caller selection")
	}
	for i, key := range []string{"compiler", "packages"} {
		p := first[i]
		if p.Attachment.Key != key || p.Attachment.Resource != p.Resource.Ref() || p.Resource.EnvironmentInstance != request.InstanceID || p.Resource.State != "planned" || p.Resource.NativeRef != "pool/"+p.Resource.Owner || p.Resource.Ref() == second[i].Resource.Ref() {
			t.Fatal("plan lost ordering, exact owner, or fresh identity", p, second[i])
		}
		if _, err := b.store.GetPersistentResource(ctx, p.Resource.ID); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("planning reserved provider data", err)
		}
	}
	if len(b.resources) != 0 || b.plans != 4 {
		t.Fatal("planning allocated data", b.resources, b.plans)
	}
	lease, plans := reserveEnvironmentContract(t, svc, b.store, request, selections)
	areas, err := svc.MaterializeEnvironmentResources(ctx, lease)
	if err != nil || len(areas) != len(plans) {
		t.Fatal(areas, err)
	}
	for i, area := range areas {
		held, err := b.store.GetPersistentResource(ctx, area.Resource.ID)
		if err != nil || held != area.Resource || held.State != "ready" || area.Attachment != plans[i].Attachment {
			t.Fatal("ready data lost reservation or receipt", area, held, err)
		}
	}
	if _, err := svc.MaterializeEnvironmentResources(ctx, lease); err == nil || len(b.resources) != 2 {
		t.Fatal("duplicate materialization replaced owned data", err)
	}
}

func TestEnvironmentResourcePlanRejectsInvalidSelectionBeforeNativePlanning(t *testing.T) {
	for _, mode := range []string{"missing-env", "invalid-instance", "too-many", "duplicate-key", "overlap", "invalid-origin", "plan-failed", "empty-native"} {
		t.Run(mode, func(t *testing.T) {
			svc, b, request, selections := environmentContractService(t)
			want := core.ErrInvalidArgument
			switch mode {
			case "missing-env":
				request.EnvironmentID = ""
			case "invalid-instance":
				request.InstanceID = "name-only"
			case "too-many":
				selections = make([]core.EnvironmentResourceSelection, core.MaxEnvironmentAttachments+1)
			case "duplicate-key":
				selections[1] = selections[0]
			case "overlap":
				selections[1].Target = selections[0].Target + "/nested"
			case "invalid-origin":
				selections[1].Origin.Epoch = ""
			case "plan-failed":
				b.planErr = core.ErrRuntimeUnavailable
				want = b.planErr
			case "empty-native":
				b.emptyPlan = true
				want = core.ErrIncompatibleState
			}
			wantAttempts := 0
			if mode == "plan-failed" || mode == "empty-native" {
				wantAttempts = 1
			}
			if plans, err := svc.PlanEnvironmentResources(context.Background(), request, selections); !errors.Is(err, want) || len(plans) != 0 || b.planAttempts != wantAttempts || len(b.resources) != 0 {
				t.Fatal("invalid plan acquired authority", plans, err, b.plans)
			}
		})
	}
}

type resourceReceiptFaultStore struct {
	*state.EnvironmentJSONStore
	stage      string
	afterWrite bool
	failure    error
}

func (s *resourceReceiptFaultStore) RecordEnvironmentResourceCreated(ctx context.Context, r core.PersistentResource) (core.PersistentResource, error) {
	if s.stage != "receipt" {
		return s.EnvironmentJSONStore.RecordEnvironmentResourceCreated(ctx, r)
	}
	if s.afterWrite {
		if _, err := s.EnvironmentJSONStore.RecordEnvironmentResourceCreated(ctx, r); err != nil {
			return r, err
		}
	}
	return r, s.failure
}

func (s *resourceReceiptFaultStore) CommitPersistentResourceCreate(ctx context.Context, r core.PersistentResource) error {
	if s.stage != "commit" {
		return s.EnvironmentJSONStore.CommitPersistentResourceCreate(ctx, r)
	}
	if s.afterWrite {
		if err := s.EnvironmentJSONStore.CommitPersistentResourceCreate(ctx, r); err != nil {
			return err
		}
	}
	return s.failure
}

func (s *resourceReceiptFaultStore) FinalizePersistentResourceDelete(ctx context.Context, r core.PersistentResource) error {
	if s.stage == "delete" {
		return s.failure
	}
	return s.EnvironmentJSONStore.FinalizePersistentResourceDelete(ctx, r)
}

func TestEnvironmentResourcePersistenceFailureKeepsExactOwnerAndParent(t *testing.T) {
	for _, stage := range []string{"receipt", "commit"} {
		for _, after := range []bool{false, true} {
			t.Run(stage+map[bool]string{false: "-before", true: "-after"}[after], func(t *testing.T) {
				svc, b, request, selections := environmentContractService(t)
				ctx := context.Background()
				lease, plans := reserveEnvironmentContract(t, svc, b.store, request, selections[:1])
				failure := errors.New("catalog write outcome uncertain")
				svc.Store = &resourceReceiptFaultStore{EnvironmentJSONStore: b.store, stage: stage, afterWrite: after, failure: failure}
				if _, err := svc.MaterializeEnvironmentResources(ctx, lease); !errors.Is(err, failure) || !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("lost persistence failure", err)
				}
				reopened := state.NewEnvironmentJSONStore(b.path)
				held, err := reopened.GetPersistentResource(ctx, plans[0].Resource.ID)
				if err != nil || held.Ref() != plans[0].Resource.Ref() || b.resources[held.ID].Ref() != held.Ref() || b.deletes != 0 {
					t.Fatal("write failure discarded exact owner", held, err)
				}
				want := "creating"
				if stage == "receipt" && after || stage == "commit" {
					want = "created"
				}
				if stage == "commit" && after {
					want = "ready"
				}
				if held.State != want {
					t.Fatal("wrong durable receipt state", held.State, want)
				}
				parent, err := reopened.GetWorkspaceLease(ctx, lease.EnvironmentID)
				if err != nil || !parent.Equal(lease) {
					t.Fatal("failed receipt lost parent reservation", parent, err)
				}
				if err := reopened.FinalizeEnvironmentDelete(ctx, lease.EnvironmentID); err == nil {
					t.Fatal("owned native child did not protect parent")
				}
			})
		}
	}
}

func TestResourceDeletionFinalizationFailureRetainsReceiptForRetry(t *testing.T) {
	svc, b, _, _ := environmentContractService(t)
	ctx := context.Background()
	r, err := svc.Create(ctx, "oci:owned", "oci-containerd")
	if err != nil {
		t.Fatal(err)
	}
	failure := errors.New("catalog finalize failed")
	store := &resourceReceiptFaultStore{EnvironmentJSONStore: b.store, stage: "delete", failure: failure}
	svc.Store = store
	if err := svc.Delete(ctx, r.ID); !errors.Is(err, failure) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	held, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || held.Ref() != r.Ref() || held.State != "deleting" || len(b.resources) != 0 {
		t.Fatal("failed finalization lost absence receipt", held, err)
	}
	store.stage = ""
	if err := svc.Delete(ctx, r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := b.store.GetPersistentResource(ctx, r.ID); !errors.Is(err, core.ErrNotFound) || b.deletes != 2 {
		t.Fatal("retry did not finalize exact owner", err, b.deletes)
	}
}
