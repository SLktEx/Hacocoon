package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type fakeEnvironmentRuntime struct {
	createSpec    core.EnvironmentRuntimeSpec
	createResult  core.EnvironmentRuntime
	createErr     error
	execRef       string
	execRequest   core.ExecutionRequest
	execResult    core.ExecutionResult
	execErr       error
	shellRef      string
	shellErr      error
	deleteRefs    []string
	deleteErr     error
	deleteFunc    func(context.Context, string) error
	cleanupCtxErr error
}

func (f *fakeEnvironmentRuntime) CreateEnvironment(_ context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	f.createSpec = spec
	return f.createResult, f.createErr
}

func (f *fakeEnvironmentRuntime) ExecEnvironment(_ context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	f.execRef = ref
	f.execRequest = req
	return f.execResult, f.execErr
}

func (f *fakeEnvironmentRuntime) ShellEnvironment(_ context.Context, ref string) error {
	f.shellRef = ref
	return f.shellErr
}

func (f *fakeEnvironmentRuntime) DeleteEnvironment(ctx context.Context, ref string) error {
	f.deleteRefs = append(f.deleteRefs, ref)
	f.cleanupCtxErr = ctx.Err()
	if f.deleteFunc != nil {
		return f.deleteFunc(ctx, ref)
	}
	return f.deleteErr
}

type fakeEnvironmentStore struct {
	environments   map[string]core.Environment
	leases         map[string]core.WorkspaceLease
	getErr         error
	putErr         error
	deleteErr      error
	leaseErr       error
	getLeaseErr    error
	putLeaseErr    error
	deleteLeaseErr error
	deleted        []string
}

func newFakeEnvironmentStore() *fakeEnvironmentStore {
	return &fakeEnvironmentStore{environments: map[string]core.Environment{}, leases: map[string]core.WorkspaceLease{}}
}

func (f *fakeEnvironmentStore) GetEnvironment(_ context.Context, name string) (core.Environment, error) {
	if f.getErr != nil {
		return core.Environment{}, f.getErr
	}
	environment, ok := f.environments[name]
	if !ok {
		return core.Environment{}, core.ErrNotFound
	}
	return environment, nil
}

func (f *fakeEnvironmentStore) PutEnvironment(_ context.Context, environment core.Environment) error {
	if f.putErr != nil {
		return f.putErr
	}
	f.environments[environment.Name] = environment
	return nil
}

func (f *fakeEnvironmentStore) DeleteEnvironment(_ context.Context, name string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, name)
	delete(f.environments, name)
	return nil
}

func (f *fakeEnvironmentStore) AcquireWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
	if f.leaseErr != nil {
		return f.leaseErr
	}
	if _, ok := f.leases[lease.EnvironmentID]; ok {
		return core.ErrAlreadyExists
	}
	for _, existing := range f.leases {
		if existing.WorkspaceID == lease.WorkspaceID &&
			(existing.AccessMode == core.WorkspaceReadWrite || lease.AccessMode == core.WorkspaceReadWrite) {
			return core.ErrWorkspaceBusy
		}
	}
	f.leases[lease.EnvironmentID] = lease
	return nil
}

func (f *fakeEnvironmentStore) GetWorkspaceLease(_ context.Context, environmentID string) (core.WorkspaceLease, error) {
	if f.getLeaseErr != nil {
		return core.WorkspaceLease{}, f.getLeaseErr
	}
	lease, ok := f.leases[environmentID]
	if !ok {
		return core.WorkspaceLease{}, core.ErrNotFound
	}
	return lease, nil
}

func (f *fakeEnvironmentStore) PutWorkspaceLease(_ context.Context, lease core.WorkspaceLease) error {
	if f.putLeaseErr != nil {
		return f.putLeaseErr
	}
	if _, ok := f.leases[lease.EnvironmentID]; !ok {
		return core.ErrNotFound
	}
	f.leases[lease.EnvironmentID] = lease
	return nil
}

func (f *fakeEnvironmentStore) DeleteWorkspaceLease(_ context.Context, environmentID string) error {
	if f.deleteLeaseErr != nil {
		return f.deleteLeaseErr
	}
	if _, ok := f.leases[environmentID]; !ok {
		return core.ErrNotFound
	}
	delete(f.leases, environmentID)
	return nil
}

func TestCreateResolvesWorkspaceAndPersistsEnvironment(t *testing.T) {
	root := t.TempDir()
	workspaceDir := filepath.Join(root, "workspace")
	if err := os.Mkdir(workspaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "workspace-link")
	if err := os.Symlink(workspaceDir, link); err != nil {
		t.Fatal(err)
	}

	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
	store := newFakeEnvironmentStore()
	service := New(runtime, store)
	fixed := time.Date(2026, 8, 29, 6, 30, 0, 0, time.UTC)
	service.now = func() time.Time { return fixed }

	environment, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: link})
	if err != nil {
		t.Fatal(err)
	}
	if runtime.createSpec.Name != "demo" || runtime.createSpec.WorkspacePath != workspaceDir || runtime.createSpec.ReadOnly {
		t.Fatalf("runtime spec = %#v", runtime.createSpec)
	}
	if environment.Name != "demo" || environment.RuntimeRef != "haco-demo" || environment.Workspace.Path != workspaceDir || environment.Workspace.ID == "" || environment.AccessMode != core.WorkspaceReadWrite || !environment.CreatedAt.Equal(fixed) {
		t.Fatalf("environment = %#v", environment)
	}
	if got := store.environments["demo"]; !reflect.DeepEqual(got, environment) {
		t.Fatalf("stored = %#v, want %#v", got, environment)
	}
	lease := store.leases["demo"]
	if lease.RuntimeRef != "haco-demo" || lease.State != core.WorkspaceLeaseActive {
		t.Fatalf("lease = %#v", lease)
	}
}

func TestCreateRejectsInvalidInputsBeforeRuntime(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []core.EnvironmentSpec{
		{Name: "Bad_Name", WorkspacePath: root},
		{Name: "demo", WorkspacePath: ""},
		{Name: "demo", WorkspacePath: filepath.Join(root, "missing")},
		{Name: "demo", WorkspacePath: file},
	}
	for _, spec := range cases {
		t.Run(spec.Name+spec.WorkspacePath, func(t *testing.T) {
			runtime := &fakeEnvironmentRuntime{}
			_, err := New(runtime, newFakeEnvironmentStore()).Create(context.Background(), spec)
			if err == nil {
				t.Fatal("expected error")
			}
			if runtime.createSpec != (core.EnvironmentRuntimeSpec{}) {
				t.Fatalf("runtime called with %#v", runtime.createSpec)
			}
		})
	}
}

func TestCreateRefusesExistingEnvironment(t *testing.T) {
	root := t.TempDir()
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}
	runtime := &fakeEnvironmentRuntime{}

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, core.ErrAlreadyExists) {
		t.Fatalf("error = %v", err)
	}
	if runtime.createSpec != (core.EnvironmentRuntimeSpec{}) {
		t.Fatalf("runtime called with %#v", runtime.createSpec)
	}
}

func TestCreateCleansRuntimeWhenPersistenceFailsEvenAfterCancellation(t *testing.T) {
	root := t.TempDir()
	persistErr := errors.New("disk full")
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
	store := newFakeEnvironmentStore()
	store.putErr = persistErr
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := New(runtime, store).Create(ctx, core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, persistErr) {
		t.Fatalf("error = %v", err)
	}
	if !reflect.DeepEqual(runtime.deleteRefs, []string{"haco-demo"}) {
		t.Fatalf("cleanup refs = %#v", runtime.deleteRefs)
	}
	if runtime.cleanupCtxErr != nil {
		t.Fatalf("cleanup context remained canceled: %v", runtime.cleanupCtxErr)
	}
	if len(store.leases) != 0 {
		t.Fatalf("lease remains after confirmed runtime cleanup: %#v", store.leases)
	}
}

func TestCreateKeepsLeaseWhenRuntimeCleanupFails(t *testing.T) {
	root := t.TempDir()
	persistErr := errors.New("disk full")
	cleanupErr := errors.New("incus delete failed")
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}, deleteErr: cleanupErr}
	store := newFakeEnvironmentStore()
	store.putErr = persistErr

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, persistErr) || !errors.Is(err, cleanupErr) || !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	lease, ok := store.leases["demo"]
	if !ok {
		t.Fatal("lease was released even though runtime cleanup failed")
	}
	if lease.RuntimeRef != "haco-demo" || lease.State != core.WorkspaceLeaseCleanupRequired {
		t.Fatalf("recovery lease = %#v", lease)
	}
}

func TestCreateCleanupHasOwnDeadline(t *testing.T) {
	root := t.TempDir()
	persistErr := errors.New("disk full")
	runtime := &fakeEnvironmentRuntime{
		createResult: core.EnvironmentRuntime{Ref: "haco-demo"},
		deleteFunc: func(ctx context.Context, _ string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	store := newFakeEnvironmentStore()
	store.putErr = persistErr
	service := New(runtime, store)
	service.cleanupTimeout = 20 * time.Millisecond

	started := time.Now()
	_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("cleanup exceeded bounded deadline: %v", elapsed)
	}
	if _, ok := store.leases["demo"]; !ok {
		t.Fatal("lease was released after timed-out runtime cleanup")
	}
}

func TestCreateReadOnlyAcquiresReadOnlyLease(t *testing.T) {
	root := t.TempDir()
	runtime := &fakeEnvironmentRuntime{createResult: core.EnvironmentRuntime{Ref: "haco-demo"}}
	store := newFakeEnvironmentStore()
	service := New(runtime, store)

	environment, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root, AccessMode: core.WorkspaceReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	if !runtime.createSpec.ReadOnly || environment.AccessMode != core.WorkspaceReadOnly {
		t.Fatalf("runtime=%#v environment=%#v", runtime.createSpec, environment)
	}
	lease, ok := store.leases["demo"]
	if !ok || lease.AccessMode != core.WorkspaceReadOnly || lease.WorkspaceID != environment.Workspace.ID {
		t.Fatalf("lease = %#v", lease)
	}
}

func TestCreateReleasesLeaseWhenRuntimeCreateFails(t *testing.T) {
	root := t.TempDir()
	runtimeErr := errors.New("incus unavailable")
	runtime := &fakeEnvironmentRuntime{createErr: runtimeErr}
	store := newFakeEnvironmentStore()

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, runtimeErr) {
		t.Fatalf("error = %v", err)
	}
	if len(store.leases) != 0 {
		t.Fatalf("lease leaked after create failure: %#v", store.leases)
	}
}

func TestCreateKeepsLeaseWhenRuntimeReportsRecoveryRequired(t *testing.T) {
	root := t.TempDir()
	runtime := &fakeEnvironmentRuntime{createErr: errors.Join(errors.New("start failed"), core.ErrRecoveryRequired)}
	store := newFakeEnvironmentStore()

	_, err := New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	if _, ok := store.leases["demo"]; !ok {
		t.Fatal("lease released although runtime cleanup was uncertain")
	}
}

func TestCreateRefusesConflictingWorkspaceLease(t *testing.T) {
	root := t.TempDir()
	provider := NewExternalPathWorkspace()
	workspace, err := provider.Resolve(context.Background(), WorkspaceRequest{Path: root})
	if err != nil {
		t.Fatal(err)
	}
	store := newFakeEnvironmentStore()
	store.leases["other"] = core.WorkspaceLease{WorkspaceID: workspace.ID, EnvironmentID: "other", AccessMode: core.WorkspaceReadWrite}
	runtime := &fakeEnvironmentRuntime{}

	_, err = New(runtime, store).Create(context.Background(), core.EnvironmentSpec{Name: "demo", WorkspacePath: root})
	if !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("error = %v", err)
	}
	if runtime.createSpec != (core.EnvironmentRuntimeSpec{}) {
		t.Fatalf("runtime called with %#v", runtime.createSpec)
	}
}

func TestExecForwardsEnvironmentAndPreservesResult(t *testing.T) {
	runtimeErr := errors.New("remote exit")
	runtime := &fakeEnvironmentRuntime{
		execResult: core.ExecutionResult{ExitCode: 7, Stdout: "out", Stderr: "err"},
		execErr:    runtimeErr,
	}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}
	service := New(runtime, store)

	request := core.ExecutionRequest{Argv: []string{"printf", "%s", "hello world"}}
	result, err := service.Exec(context.Background(), "demo", request)
	if !errors.Is(err, runtimeErr) || result.ExitCode != 7 || result.Stdout != "out" || result.Stderr != "err" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if runtime.execRef != "haco-demo" || !reflect.DeepEqual(runtime.execRequest, request) {
		t.Fatalf("forwarded ref=%q request=%#v", runtime.execRef, runtime.execRequest)
	}
}

func TestDeleteKeepsMetadataWhenRuntimeDeleteFails(t *testing.T) {
	deleteErr := errors.New("incus busy")
	runtime := &fakeEnvironmentRuntime{deleteErr: deleteErr}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}

	err := New(runtime, store).Delete(context.Background(), "demo")
	if !errors.Is(err, deleteErr) {
		t.Fatalf("error = %v", err)
	}
	if _, ok := store.environments["demo"]; !ok {
		t.Fatal("metadata removed after runtime delete failure")
	}
	if len(store.deleted) != 0 {
		t.Fatalf("store delete calls = %#v", store.deleted)
	}
}

func TestDeleteTreatsMissingRuntimeAsAlreadyDeleted(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{deleteErr: core.ErrNotFound}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}
	store.leases["demo"] = core.WorkspaceLease{EnvironmentID: "demo", RuntimeRef: "haco-demo", AccessMode: core.WorkspaceReadWrite}

	if err := New(runtime, store).Delete(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.environments["demo"]; ok {
		t.Fatal("metadata remains after missing runtime was treated as deleted")
	}
	if _, ok := store.leases["demo"]; ok {
		t.Fatal("lease remains after idempotent delete")
	}
}

func TestDeleteRecoversLeaseAfterMetadataWasAlreadyRemoved(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{deleteErr: core.ErrNotFound}
	store := newFakeEnvironmentStore()
	store.leases["demo"] = core.WorkspaceLease{
		EnvironmentID: "demo",
		RuntimeRef:    "haco-demo",
		AccessMode:    core.WorkspaceReadWrite,
		State:         core.WorkspaceLeaseActive,
	}

	if err := New(runtime, store).Delete(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.deleteRefs, []string{"haco-demo"}) {
		t.Fatalf("recovery runtime deletes = %#v", runtime.deleteRefs)
	}
	if _, ok := store.leases["demo"]; ok {
		t.Fatal("lease remains after recovery delete")
	}
}

func TestDeleteRefusesLeaseRecoveryWithoutRuntimeProof(t *testing.T) {
	store := newFakeEnvironmentStore()
	store.leases["demo"] = core.WorkspaceLease{
		EnvironmentID: "demo",
		AccessMode:    core.WorkspaceReadWrite,
		State:         core.WorkspaceLeaseAcquiring,
	}

	err := New(&fakeEnvironmentRuntime{}, store).Delete(context.Background(), "demo")
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("error = %v", err)
	}
	if _, ok := store.leases["demo"]; !ok {
		t.Fatal("uncertain lease was reclaimed")
	}
}

func TestDeleteRemovesRuntimeBeforeMetadata(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}
	store.leases["demo"] = core.WorkspaceLease{EnvironmentID: "demo", RuntimeRef: "haco-demo", AccessMode: core.WorkspaceReadWrite}

	if err := New(runtime, store).Delete(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.deleteRefs, []string{"haco-demo"}) || !reflect.DeepEqual(store.deleted, []string{"demo"}) {
		t.Fatalf("runtime deletes=%#v metadata deletes=%#v", runtime.deleteRefs, store.deleted)
	}
	if _, ok := store.leases["demo"]; ok {
		t.Fatal("workspace lease remains after delete")
	}
}

func TestShellUsesStoredRuntimeReference(t *testing.T) {
	runtime := &fakeEnvironmentRuntime{}
	store := newFakeEnvironmentStore()
	store.environments["demo"] = core.Environment{Name: "demo", RuntimeRef: "haco-demo"}

	if err := New(runtime, store).Shell(context.Background(), "demo"); err != nil {
		t.Fatal(err)
	}
	if runtime.shellRef != "haco-demo" {
		t.Fatalf("shell ref = %q", runtime.shellRef)
	}
}

type blockingEnvironmentRuntime struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingEnvironmentRuntime) CreateEnvironment(_ context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if spec.Name == "one" {
		close(b.started)
		<-b.release
	}
	return core.EnvironmentRuntime{Ref: "haco-" + spec.Name}, nil
}
func (*blockingEnvironmentRuntime) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, nil
}
func (*blockingEnvironmentRuntime) ShellEnvironment(context.Context, string) error  { return nil }
func (*blockingEnvironmentRuntime) DeleteEnvironment(context.Context, string) error { return nil }

func TestCreateSerializesConcurrentWorkspaceLeaseAcquisition(t *testing.T) {
	root := t.TempDir()
	runtime := &blockingEnvironmentRuntime{started: make(chan struct{}), release: make(chan struct{})}
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	service := New(runtime, store)

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "one", WorkspacePath: root})
		firstDone <- err
	}()
	<-runtime.started

	secondDone := make(chan error, 1)
	go func() {
		_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "two", WorkspacePath: root})
		secondDone <- err
	}()

	select {
	case err := <-secondDone:
		t.Fatalf("second create escaped workspace serialization early: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(runtime.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first create: %v", err)
	}
	if err := <-secondDone; !errors.Is(err, core.ErrWorkspaceBusy) {
		t.Fatalf("second create error = %v", err)
	}
}

func TestCreateOnDifferentWorkspaceDoesNotReclaimInFlightLease(t *testing.T) {
	root := t.TempDir()
	onePath := filepath.Join(root, "one")
	twoPath := filepath.Join(root, "two")
	if err := os.Mkdir(onePath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(twoPath, 0o755); err != nil {
		t.Fatal(err)
	}
	runtime := &blockingEnvironmentRuntime{started: make(chan struct{}), release: make(chan struct{})}
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state", "environments.json"))
	service := New(runtime, store)

	firstDone := make(chan error, 1)
	go func() {
		_, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "one", WorkspacePath: onePath})
		firstDone <- err
	}()
	<-runtime.started

	if _, err := service.Create(context.Background(), core.EnvironmentSpec{Name: "two", WorkspacePath: twoPath}); err != nil {
		t.Fatalf("second workspace create: %v", err)
	}
	if _, err := store.GetWorkspaceLease(context.Background(), "one"); err != nil {
		t.Fatalf("in-flight lease was reclaimed: %v", err)
	}

	close(runtime.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first create: %v", err)
	}
}
