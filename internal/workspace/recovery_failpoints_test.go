package workspace

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type semanticFailpoint string

const (
	failBeforeBeginCreate       semanticFailpoint = "environment-create/before-reservation"
	failAfterBeginCreate        semanticFailpoint = "environment-create/after-reservation"
	failBeforeRecordRuntime     semanticFailpoint = "environment-create/before-runtime-ownership"
	failAfterRecordRuntime      semanticFailpoint = "environment-create/after-runtime-ownership"
	failBeforeCommitReady       semanticFailpoint = "environment-create/before-ready-commit"
	failAfterCommitReady        semanticFailpoint = "environment-create/after-ready-commit"
	failBeforeFinalizeDelete    semanticFailpoint = "environment-delete/before-state-finalize"
	failAfterFinalizeDelete     semanticFailpoint = "environment-delete/after-state-finalize"
	failBeforeRuntimeDelete     semanticFailpoint = "environment-delete/before-runtime-delete"
	failAfterRuntimeDelete      semanticFailpoint = "environment-delete/after-runtime-delete"
)

var errInjectedFailure = errors.New("injected reliability failure")

type failpointStore struct {
	environmentStore
	point semanticFailpoint
	fired bool
}

func (s *failpointStore) arm(point semanticFailpoint) {
	s.point = point
	s.fired = false
}

func (s *failpointStore) inject(point semanticFailpoint) error {
	if s.point != point || s.fired {
		return nil
	}
	s.fired = true
	return fmt.Errorf("%s: %w", point, errInjectedFailure)
}

func (s *failpointStore) BeginEnvironmentCreate(ctx context.Context, lease core.WorkspaceLease) error {
	if err := s.inject(failBeforeBeginCreate); err != nil {
		return err
	}
	if err := s.environmentStore.BeginEnvironmentCreate(ctx, lease); err != nil {
		return err
	}
	return s.inject(failAfterBeginCreate)
}

func (s *failpointStore) RecordEnvironmentRuntime(ctx context.Context, lease core.WorkspaceLease) error {
	if err := s.inject(failBeforeRecordRuntime); err != nil {
		return err
	}
	if err := s.environmentStore.RecordEnvironmentRuntime(ctx, lease); err != nil {
		return err
	}
	return s.inject(failAfterRecordRuntime)
}

func (s *failpointStore) CommitEnvironmentCreate(ctx context.Context, environment core.Environment, lease core.WorkspaceLease) error {
	if err := s.inject(failBeforeCommitReady); err != nil {
		return err
	}
	if err := s.environmentStore.CommitEnvironmentCreate(ctx, environment, lease); err != nil {
		return err
	}
	return s.inject(failAfterCommitReady)
}

func (s *failpointStore) FinalizeEnvironmentDelete(ctx context.Context, environmentID string) error {
	if err := s.inject(failBeforeFinalizeDelete); err != nil {
		return err
	}
	if err := s.environmentStore.FinalizeEnvironmentDelete(ctx, environmentID); err != nil {
		return err
	}
	return s.inject(failAfterFinalizeDelete)
}

type failpointRuntime struct {
	refs  map[string]bool
	point semanticFailpoint
	fired bool
}

func newFailpointRuntime() *failpointRuntime {
	return &failpointRuntime{refs: map[string]bool{}}
}

func (r *failpointRuntime) arm(point semanticFailpoint) {
	r.point = point
	r.fired = false
}

func (r *failpointRuntime) inject(point semanticFailpoint) error {
	if r.point != point || r.fired {
		return nil
	}
	r.fired = true
	return fmt.Errorf("%s: %w", point, errInjectedFailure)
}

func (r *failpointRuntime) CreateEnvironment(_ context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	ref := "haco-" + spec.Name
	r.refs[ref] = true
	return core.EnvironmentRuntime{Ref: ref, Base: spec.Base, Resources: spec.Resources}, nil
}

func (*failpointRuntime) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}

func (*failpointRuntime) ShellEnvironment(context.Context, string) error { return nil }

func (r *failpointRuntime) DeleteEnvironment(_ context.Context, ref string) error {
	if err := r.inject(failBeforeRuntimeDelete); err != nil {
		return err
	}
	if !r.refs[ref] {
		return core.ErrNotFound
	}
	delete(r.refs, ref)
	if err := r.inject(failAfterRuntimeDelete); err != nil {
		return err
	}
	return nil
}

func newRecoveryHarness(t *testing.T) (*Service, *failpointStore, *failpointRuntime, string) {
	t.Helper()
	workspacePath := filepath.Clean(t.TempDir())
	store := &failpointStore{environmentStore: state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))}
	runtime := newFailpointRuntime()
	return New(runtime, store), store, runtime, workspacePath
}

func assertReadyAggregate(t *testing.T, store environmentStore, runtime *failpointRuntime, name string) {
	t.Helper()
	environment, err := store.GetEnvironment(context.Background(), name)
	if err != nil {
		t.Fatalf("ready Environment missing: %v", err)
	}
	lease, err := store.GetWorkspaceLease(context.Background(), name)
	if err != nil {
		t.Fatalf("ready Workspace lease missing: %v", err)
	}
	if lease.State != core.WorkspaceLeaseActive || lease.RuntimeRef != environment.RuntimeRef {
		t.Fatalf("ready aggregate disagrees: environment=%#v lease=%#v", environment, lease)
	}
	if !runtime.refs[environment.RuntimeRef] {
		t.Fatalf("authoritative state points at absent runtime %q", environment.RuntimeRef)
	}
}

func assertAbsentAggregate(t *testing.T, store environmentStore, runtime *failpointRuntime, name, runtimeRef string) {
	t.Helper()
	if _, err := store.GetEnvironment(context.Background(), name); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Environment remained after recovery: %v", err)
	}
	if _, err := store.GetWorkspaceLease(context.Background(), name); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Workspace lease remained after recovery: %v", err)
	}
	if runtime.refs[runtimeRef] {
		t.Fatalf("runtime %q remained after recovery", runtimeRef)
	}
}

func TestEnvironmentCreateDurableBoundaryFailuresConvergeOnRetry(t *testing.T) {
	for _, point := range []semanticFailpoint{
		failBeforeBeginCreate,
		failBeforeRecordRuntime,
		failAfterRecordRuntime,
		failBeforeCommitReady,
		failAfterCommitReady,
	} {
		t.Run(string(point), func(t *testing.T) {
			service, store, runtime, workspacePath := newRecoveryHarness(t)
			store.arm(point)

			_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: workspacePath})
			if !errors.Is(err, errInjectedFailure) {
				t.Fatalf("first create error = %v, want injected failure", err)
			}

			// Every covered boundary either happened before provider ownership or
			// was followed by bounded cleanup, so retry must start from an empty
			// aggregate rather than adopting partial state.
			assertAbsentAggregate(t, store, runtime, "demo", "haco-demo")

			if _, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: workspacePath}); err != nil {
				t.Fatalf("retry create: %v", err)
			}
			assertReadyAggregate(t, store, runtime, "demo")
		})
	}
}

func TestEnvironmentCreateLostReservationResponseFailsClosed(t *testing.T) {
	service, store, runtime, workspacePath := newRecoveryHarness(t)
	store.arm(failAfterBeginCreate)

	_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: workspacePath})
	if !errors.Is(err, errInjectedFailure) {
		t.Fatalf("first create error = %v", err)
	}
	lease, err := store.GetWorkspaceLease(context.Background(), "demo")
	if err != nil {
		t.Fatalf("durable reservation missing: %v", err)
	}
	if lease.State != core.WorkspaceLeaseAcquiring || lease.RuntimeRef != "" {
		t.Fatalf("unexpected reservation after response loss: %#v", lease)
	}
	if runtime.refs["haco-demo"] {
		t.Fatal("runtime was created after reservation response was lost")
	}

	// A second create must not guess that the reservation is stale and must not
	// create another runtime. This is the conservative #381 state-3 outcome.
	_, retryErr := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: workspacePath})
	if !errors.Is(retryErr, core.ErrAlreadyExists) {
		t.Fatalf("retry create error = %v, want fail-closed existing reservation", retryErr)
	}
	if runtime.refs["haco-demo"] {
		t.Fatal("retry adopted ambiguous reservation and created a runtime")
	}
}

func TestEnvironmentDeleteDurableBoundaryFailuresConvergeOnRetry(t *testing.T) {
	for _, point := range []semanticFailpoint{
		failBeforeRuntimeDelete,
		failAfterRuntimeDelete,
		failBeforeFinalizeDelete,
		failAfterFinalizeDelete,
	} {
		t.Run(string(point), func(t *testing.T) {
			service, store, runtime, workspacePath := newRecoveryHarness(t)
			if _, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: workspacePath}); err != nil {
				t.Fatalf("seed Environment: %v", err)
			}
			assertReadyAggregate(t, store, runtime, "demo")

			if point == failBeforeRuntimeDelete || point == failAfterRuntimeDelete {
				runtime.arm(point)
			} else {
				store.arm(point)
			}
			err := service.Delete(context.Background(), "demo")
			if !errors.Is(err, errInjectedFailure) {
				t.Fatalf("first delete error = %v, want injected failure", err)
			}

			if err := service.Delete(context.Background(), "demo"); err != nil {
				t.Fatalf("retry delete: %v", err)
			}
			assertAbsentAggregate(t, store, runtime, "demo", "haco-demo")

			// Delete remains idempotent after recovery has converged.
			if err := service.Delete(context.Background(), "demo"); err != nil {
				t.Fatalf("idempotent delete after recovery: %v", err)
			}
		})
	}
}
