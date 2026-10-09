//go:build linux

package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

const recoveryProcessExitCode = 86

// This inventory is the test provider's external state, separate from the real
// lifecycle catalog. It survives each process exit; a new provider cannot forget
// an instance merely because the process that created it has terminated.
// Writes finish before semantic interruption. This is not power-loss or Incus
// acceptance, and the fixture never operates on real provider resources.
type recoveryProcessInventory struct {
	Resources map[string]string
	Created   []string
	Deleted   []string
}

type recoveryProcessRuntime struct {
	path      string
	inventory recoveryProcessInventory
	boundary  func(semanticFailpoint) error
}

func (r *recoveryProcessRuntime) save() error {
	data, err := json.Marshal(r.inventory)
	if err != nil {
		return err
	}
	return os.WriteFile(r.path, data, 0o600)
}

func (*recoveryProcessRuntime) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return core.EnvironmentRuntime{}, core.ErrUnsupported // Require the production receipt contract.
}

func (r *recoveryProcessRuntime) CreateEnvironmentWithReceipt(_ context.Context, spec core.EnvironmentRuntimeSpec, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	created := core.EnvironmentRuntime{Ref: "fixture-" + spec.InstanceID, Resources: spec.Resources}
	if !core.ValidEnvironmentInstanceID(spec.InstanceID) {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	r.inventory.Resources[created.Ref] = spec.InstanceID
	r.inventory.Created = append(r.inventory.Created, created.Ref)
	if err := r.save(); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	return created, record(created)
}

func (*recoveryProcessRuntime) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, core.ErrUnsupported
}

func (*recoveryProcessRuntime) ShellEnvironment(context.Context, string) error {
	return core.ErrUnsupported
}

func (r *recoveryProcessRuntime) DeleteEnvironment(_ context.Context, ref string) error {
	if err := r.boundary(failBeforeRuntimeDelete); err != nil {
		return err
	}
	_, present := r.inventory.Resources[ref]
	r.inventory.Deleted = append(r.inventory.Deleted, ref)
	delete(r.inventory.Resources, ref)
	if err := r.save(); err != nil {
		return err
	}
	if err := r.boundary(failAfterRuntimeDelete); err != nil {
		return err
	}
	if !present {
		return core.ErrNotFound
	}
	return nil
}

func openRecoveryProcessFixture(t *testing.T, root string) (*Service, *failpointStore, *recoveryProcessRuntime) {
	t.Helper()
	store := &failpointStore{environmentStore: state.NewEnvironmentJSONStore(filepath.Join(root, "catalog", "environments.json"))}
	provider := &recoveryProcessRuntime{path: filepath.Join(root, "provider.json"), boundary: store.inject}
	data, err := os.ReadFile(provider.path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &provider.inventory); err != nil {
		t.Fatal(err)
	}
	return New(provider, store), store, provider
}

// Only the test binary contains this entry and its interruption switch. os.Exit
// deliberately bypasses all cleanup and unlock defers in the lifecycle caller.
func TestEnvironmentRecoveryProcessChild(t *testing.T) {
	root := os.Getenv("HACO_TEST_RECOVERY_ROOT")
	if root == "" {
		return
	}
	point := semanticFailpoint(os.Getenv("HACO_TEST_RECOVERY_POINT"))
	operation := os.Getenv("HACO_TEST_RECOVERY_OPERATION")
	if os.Getenv("HACO_TEST_RECOVERY_RESTART") == "1" {
		verifyRecoveryProcessRestart(t, root, operation, point)
		return
	}
	service, store, _ := openRecoveryProcessFixture(t, root)
	store.arm(point)
	store.interrupt = func() { os.Exit(recoveryProcessExitCode) }
	if operation == "create" {
		_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: filepath.Join(root, "workspace")})
		t.Fatalf("create returned instead of terminating at %s: %v", point, err)
	}
	if operation == "delete" {
		err := service.Delete(context.Background(), "demo")
		t.Fatalf("delete returned instead of terminating at %s: %v", point, err)
	}
	t.Fatalf("unknown recovery operation %q", operation)
}

func runRecoveryProcess(t *testing.T, root, operation string, point semanticFailpoint, restart bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestEnvironmentRecoveryProcessChild$")
	cmd.Env = append(os.Environ(),
		"HACO_TEST_RECOVERY_ROOT="+root,
		"HACO_TEST_RECOVERY_OPERATION="+operation,
		"HACO_TEST_RECOVERY_POINT="+string(point),
		"HACO_TEST_RECOVERY_RESTART=0",
	)
	if restart {
		cmd.Env = append(cmd.Env, "HACO_TEST_RECOVERY_RESTART=1")
	}
	output, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("process failed to finish (including lifecycle lock recovery): %v\n%s", ctx.Err(), output)
	}
	if restart {
		if err != nil {
			t.Fatalf("fresh-process recovery: %v\n%s", err, output)
		}
		return
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != recoveryProcessExitCode {
		t.Fatalf("process did not exit at %s: %v\n%s", point, err, output)
	}
}

func TestEnvironmentLifecycleProcessExitRecovery(t *testing.T) {
	for _, tc := range []struct {
		operation string
		point     semanticFailpoint
	}{
		{"create", failBeforeBeginCreate},
		{"create", failAfterBeginCreate},
		{"create", failBeforeRecordRuntime},
		{"create", failAfterRecordRuntime},
		{"create", failBeforeCommitReady},
		{"create", failAfterCommitReady},
		{"delete", failBeforeRuntimeDelete},
		{"delete", failAfterRuntimeDelete},
		{"delete", failBeforeFinalizeDelete},
		{"delete", failAfterFinalizeDelete},
	} {
		t.Run(string(tc.point), func(t *testing.T) {
			root := t.TempDir()
			workspace := filepath.Join(root, "workspace")
			if err := os.Mkdir(workspace, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(workspace, "retained"), []byte("user data"), 0o600); err != nil {
				t.Fatal(err)
			}
			provider := &recoveryProcessRuntime{path: filepath.Join(root, "provider.json"), inventory: recoveryProcessInventory{Resources: map[string]string{"unrelated": "foreign-owner"}}}
			if err := provider.save(); err != nil {
				t.Fatal(err)
			}
			if tc.operation == "delete" {
				service, _, _ := openRecoveryProcessFixture(t, root)
				if _, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: workspace}); err != nil {
					t.Fatal(err)
				}
			}
			runRecoveryProcess(t, root, tc.operation, tc.point, false)
			runRecoveryProcess(t, root, tc.operation, tc.point, true)
		})
	}
}

func verifyRecoveryProcessRestart(t *testing.T, root, operation string, point semanticFailpoint) {
	t.Helper()
	service, store, provider := openRecoveryProcessFixture(t, root)
	ctx := context.Background()
	workspace := filepath.Join(root, "workspace")
	wantLease := point != failBeforeBeginCreate && point != failAfterFinalizeDelete
	wantReady := wantLease && (operation == "delete" || point == failAfterCommitReady)
	wantRef := operation == "delete" || point == failAfterRecordRuntime || point == failBeforeCommitReady || point == failAfterCommitReady
	wantRuntime := point != failBeforeBeginCreate && point != failAfterBeginCreate && point != failAfterRuntimeDelete && point != failBeforeFinalizeDelete && point != failAfterFinalizeDelete
	lease, leaseErr := store.GetWorkspaceLease(ctx, "demo")
	environment, envErr := store.GetEnvironment(ctx, "demo")
	if wantLease {
		if leaseErr != nil || !core.ValidEnvironmentInstanceID(lease.InstanceID) || lease.Owner != "demo" || lease.EnvironmentID != "demo" || lease.SourcePath != workspace || lease.AccessMode != core.WorkspaceReadWrite || lease.RuntimeAbsent {
			t.Fatalf("lost or changed durable lease: %#v, %v", lease, leaseErr)
		}
		resolved, err := NewExternalPathWorkspace().Resolve(ctx, WorkspaceRequest{Path: workspace})
		if err != nil || lease.WorkspaceID != resolved.ID {
			t.Fatalf("Workspace identity changed: %#v, %v", lease, err)
		}
		wantState := core.WorkspaceLeaseAcquiring
		if wantReady {
			wantState = core.WorkspaceLeaseActive
		}
		if lease.State != wantState {
			t.Fatalf("lease state = %s, want %s", lease.State, wantState)
		}
	} else if !errors.Is(leaseErr, core.ErrNotFound) {
		t.Fatalf("unexpected lease after %s: %#v, %v", point, lease, leaseErr)
	}
	if wantReady {
		if envErr != nil || !lease.MatchesEnvironment(environment) {
			t.Fatalf("Ready publication is not atomic: %#v, %v", environment, envErr)
		}
	} else if !errors.Is(envErr, core.ErrNotFound) {
		t.Fatalf("unexpected Ready publication: %#v, %v", environment, envErr)
	}
	var exactRef string
	if len(provider.inventory.Created) != 0 {
		if len(provider.inventory.Created) != 1 {
			t.Fatalf("unexpected provider creations: %#v", provider.inventory)
		}
		exactRef = provider.inventory.Created[0]
	}
	if wantLease && wantRef && (exactRef == "" || lease.RuntimeRef != exactRef || exactRef != "fixture-"+lease.InstanceID) {
		t.Fatalf("exact provider ownership did not survive exit: lease=%#v provider=%#v", lease, provider.inventory)
	}
	if wantLease && !wantRef && lease.RuntimeRef != "" {
		t.Fatalf("guessed ownership before receipt: %#v", lease)
	}
	wantResources := map[string]string{"unrelated": "foreign-owner"}
	if wantRuntime {
		if exactRef == "" || exactRef != "fixture-"+lease.InstanceID {
			t.Fatalf("created resource identity disagrees with reservation: %#v, %#v", lease, provider.inventory)
		}
		wantResources[exactRef] = lease.InstanceID
	}
	if !reflect.DeepEqual(provider.inventory.Resources, wantResources) {
		t.Fatalf("provider state did not survive exit: %#v, want %#v", provider.inventory.Resources, wantResources)
	}
	wantDeletes := 0
	if operation == "delete" && !wantRuntime {
		wantDeletes = 1
	}
	if len(provider.inventory.Deleted) != wantDeletes || (wantDeletes == 1 && provider.inventory.Deleted[0] != exactRef) {
		t.Fatalf("unexpected deletion before restart: %#v", provider.inventory)
	}

	before, err := os.ReadFile(provider.path)
	if err != nil {
		t.Fatal(err)
	}
	if wantLease {
		if _, err := service.Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: workspace}); !errors.Is(err, core.ErrAlreadyExists) {
			t.Fatalf("same-name retry adopted retained state: %v", err)
		}
		if _, err := service.Create(ctx, core.EnvironmentSpec{Name: "conflict", WorkspacePath: workspace}); !errors.Is(err, core.ErrWorkspaceBusy) {
			t.Fatalf("retained RW lease did not exclude another Environment: %v", err)
		}
		after, err := os.ReadFile(provider.path)
		retained, leaseErr := store.GetWorkspaceLease(ctx, "demo")
		if err != nil || string(after) != string(before) || leaseErr != nil || !retained.Equal(lease) {
			t.Fatalf("refused creation changed retained ownership: %v, %v", err, leaseErr)
		}
	}
	if wantLease && !wantRef {
		if err := service.Delete(ctx, "demo"); !errors.Is(err, core.ErrRecoveryRequired) {
			t.Fatalf("unreceipted reservation was reclaimed without proof: %v", err)
		}
		after, err := os.ReadFile(provider.path)
		retained, leaseErr := store.GetWorkspaceLease(ctx, "demo")
		if err != nil || string(after) != string(before) || leaseErr != nil || !retained.Equal(lease) {
			t.Fatalf("ambiguous ownership changed during retry: %v, %v", err, leaseErr)
		}
	} else {
		for attempt := range 2 {
			deletes := len(provider.inventory.Deleted)
			if err := service.Delete(ctx, "demo"); err != nil {
				t.Fatalf("exact-owner cleanup/idempotent retry: %v", err)
			}
			if attempt == 1 && len(provider.inventory.Deleted) != deletes {
				t.Fatal("idempotent deletion reached the provider after finalization")
			}
		}
		service, store, provider = openRecoveryProcessFixture(t, root)
		if _, err := store.GetWorkspaceLease(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("cleanup retained lease: %v", err)
		}
		if _, err := store.GetEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("cleanup retained Environment: %v", err)
		}
		if !reflect.DeepEqual(provider.inventory.Resources, map[string]string{"unrelated": "foreign-owner"}) {
			t.Fatalf("cleanup lost unrelated resource or retained owned resource: %#v", provider.inventory)
		}
		for _, deleted := range provider.inventory.Deleted {
			if exactRef == "" || deleted != exactRef {
				t.Fatalf("cleanup targeted a guessed/unrelated resource: %q, want %q", deleted, exactRef)
			}
		}
		creates := len(provider.inventory.Created)
		recreated, err := service.Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: workspace})
		if err != nil {
			t.Fatalf("safe creation retry after completed cleanup: %v", err)
		}
		fresh, err := store.GetWorkspaceLease(ctx, "demo")
		if err != nil || !core.ValidEnvironmentInstanceID(fresh.InstanceID) || fresh.State != core.WorkspaceLeaseActive || !fresh.MatchesEnvironment(recreated) || fresh.SourcePath != workspace || fresh.AccessMode != core.WorkspaceReadWrite || fresh.Owner != "demo" {
			t.Fatalf("recreation did not publish a Ready aggregate: %#v, %#v, %v", recreated, fresh, err)
		}
		newRef := "fixture-" + fresh.InstanceID
		if recreated.RuntimeRef != newRef || newRef == exactRef || len(provider.inventory.Created) != creates+1 || provider.inventory.Created[creates] != newRef {
			t.Fatalf("recreation reused or changed provider identity: %#v, %#v", fresh, provider.inventory)
		}
		published, err := store.GetEnvironment(ctx, "demo")
		if err != nil || !published.Equal(recreated) || !reflect.DeepEqual(provider.inventory.Resources, map[string]string{"unrelated": "foreign-owner", newRef: fresh.InstanceID}) {
			t.Fatalf("recreated catalog/provider disagree: %#v, %#v, %v", published, provider.inventory, err)
		}
		deletes := len(provider.inventory.Deleted)
		if err := service.Delete(ctx, "demo"); err != nil {
			t.Fatal(err)
		}
		if err := service.Delete(ctx, "demo"); err != nil {
			t.Fatal(err)
		}
		_, store, provider = openRecoveryProcessFixture(t, root)
		if len(provider.inventory.Deleted) != deletes+1 || provider.inventory.Deleted[deletes] != newRef || !reflect.DeepEqual(provider.inventory.Resources, map[string]string{"unrelated": "foreign-owner"}) {
			t.Fatalf("recreated cleanup changed unrelated resources or repeated provider deletion: %#v", provider.inventory)
		}
		if _, err := store.GetEnvironment(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("recreated Environment survived cleanup: %v", err)
		}
		if _, err := store.GetWorkspaceLease(ctx, "demo"); !errors.Is(err, core.ErrNotFound) {
			t.Fatalf("recreated lease survived cleanup: %v", err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(workspace, "retained")); err != nil || string(data) != "user data" {
		t.Fatalf("external Workspace data changed: %q, %v", data, err)
	}
}
