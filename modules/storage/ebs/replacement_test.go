package ebs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type fakeAPI struct {
	calls        []string
	fail         string
	createTokens []string
	volumes      map[string]Volume
}

func (f *fakeAPI) call(name string) error {
	f.calls = append(f.calls, name)
	if f.fail == name {
		return errors.New("forced " + name)
	}
	return nil
}

func (f *fakeAPI) ensureVolumes() {
	if f.volumes != nil {
		return
	}
	f.volumes = map[string]Volume{
		"vol-source": {
			ID:               "vol-source",
			SizeGiB:          100,
			AvailabilityZone: "ap-northeast-1a",
			Type:             "gp3",
			Encrypted:        true,
			State:            "in-use",
			Attachments: []VolumeAttachment{{
				InstanceID: "i-0123456789abcdef0",
				Device:     "/dev/sdf",
				State:      "attached",
			}},
		},
	}
}

func (f *fakeAPI) DescribeVolume(_ context.Context, id string) (Volume, error) {
	f.ensureVolumes()
	if err := f.call("describe:" + id); err != nil {
		return Volume{}, err
	}
	volume, ok := f.volumes[id]
	if !ok {
		return Volume{}, core.ErrNotFound
	}
	return volume, nil
}

func (f *fakeAPI) CreateVolume(_ context.Context, source Volume, size int64, clientToken string) (string, error) {
	f.ensureVolumes()
	f.createTokens = append(f.createTokens, clientToken)
	if err := f.call("create"); err != nil {
		return "", err
	}
	f.volumes["vol-target"] = Volume{
		ID:               "vol-target",
		SizeGiB:          size,
		AvailabilityZone: source.AvailabilityZone,
		Type:             source.Type,
		Encrypted:        source.Encrypted,
		State:            "available",
	}
	return "vol-target", nil
}

func (f *fakeAPI) AttachVolume(_ context.Context, volume, instance, device string) error {
	f.ensureVolumes()
	if err := f.call("attach:" + volume + ":" + device); err != nil {
		return err
	}
	v := f.volumes[volume]
	v.State = "in-use"
	v.Attachments = []VolumeAttachment{{InstanceID: instance, Device: device, State: "attached"}}
	f.volumes[volume] = v
	return nil
}

func (f *fakeAPI) DetachVolume(_ context.Context, volume, _ string) error {
	f.ensureVolumes()
	if err := f.call("detach:" + volume); err != nil {
		return err
	}
	v := f.volumes[volume]
	v.State = "available"
	v.Attachments = nil
	f.volumes[volume] = v
	return nil
}

func (f *fakeAPI) WaitAvailable(_ context.Context, volume string) error {
	return f.call("available:" + volume)
}

func (f *fakeAPI) WaitInUse(_ context.Context, volume string) error {
	return f.call("in-use:" + volume)
}

func (f *fakeAPI) DeleteVolume(_ context.Context, volume string) error {
	f.ensureVolumes()
	if err := f.call("delete:" + volume); err != nil {
		return err
	}
	delete(f.volumes, volume)
	return nil
}

type fakeMigrator struct {
	calls []string
	fail  string
}

func (f *fakeMigrator) step(name string) error {
	f.calls = append(f.calls, name)
	if f.fail == name {
		return errors.New("forced " + name)
	}
	return nil
}

func (f *fakeMigrator) Preflight(context.Context, ReplacementSpec, Volume) error {
	return f.step("preflight")
}

func (f *fakeMigrator) Migrate(context.Context, ReplacementSpec, string) error {
	return f.step("migrate")
}

func (f *fakeMigrator) Verify(context.Context, ReplacementSpec, string) error {
	return f.step("verify")
}

func (f *fakeMigrator) Activate(context.Context, ReplacementSpec, string) error {
	return f.step("activate")
}

type fakeJournal struct {
	saved     []Operation
	deleted   []string
	failPhase Phase
	current   map[string]Operation
	lockErr   error
}

func (f *fakeJournal) Save(_ context.Context, op Operation) error {
	f.saved = append(f.saved, op)
	if op.Phase == f.failPhase {
		return errors.New("journal failed")
	}
	if f.current == nil {
		f.current = map[string]Operation{}
	}
	f.current[op.ID] = op
	return nil
}

func (f *fakeJournal) Load(_ context.Context, id string) (Operation, bool, error) {
	op, ok := f.current[id]
	return op, ok, nil
}

func (f *fakeJournal) List(context.Context) ([]Operation, error) {
	operations := make([]Operation, 0, len(f.current))
	for _, op := range f.current {
		operations = append(operations, op)
	}
	sort.Slice(operations, func(i, j int) bool { return operations[i].ID < operations[j].ID })
	return operations, nil
}

func (f *fakeJournal) Delete(_ context.Context, id string) error {
	f.deleted = append(f.deleted, id)
	delete(f.current, id)
	return nil
}

func (f *fakeJournal) Lock(context.Context, ...string) (func() error, error) {
	if f.lockErr != nil {
		return nil, f.lockErr
	}
	return func() error { return nil }, nil
}

func spec() ReplacementSpec {
	return ReplacementSpec{
		OperationID:    "resize-1",
		InstanceID:     "i-0123456789abcdef0",
		SourceVolumeID: "vol-source",
		SourceDevice:   "/dev/sdf",
		StagingDevice:  "/dev/sdg",
		TargetSizeGiB:  60,
	}
}

func TestReplacementMigratesVerifiesAndPromotesWithoutDeletingSource(t *testing.T) {
	api := &fakeAPI{}
	mig := &fakeMigrator{}
	journal := &fakeJournal{}
	op, err := New(api, mig, journal).Replace(context.Background(), spec())
	if err != nil {
		t.Fatal(err)
	}
	if op.Phase != PhaseComplete || op.TargetVolumeID != "vol-target" {
		t.Fatalf("op=%#v", op)
	}
	wantAPI := []string{
		"describe:vol-source",
		"create",
		"available:vol-target",
		"attach:vol-target:/dev/sdg",
		"in-use:vol-target",
		"describe:vol-target",
		"detach:vol-source",
		"available:vol-source",
		"describe:vol-target",
		"detach:vol-target",
		"available:vol-target",
		"attach:vol-target:/dev/sdf",
		"in-use:vol-target",
	}
	if strings.Join(api.calls, "|") != strings.Join(wantAPI, "|") {
		t.Fatalf("api calls=%v", api.calls)
	}
	if len(api.createTokens) != 1 || api.createTokens[0] != clientTokenForOperation("resize-1") {
		t.Fatalf("create tokens=%v", api.createTokens)
	}
	if strings.Join(mig.calls, "|") != "preflight|migrate|verify|activate" {
		t.Fatalf("migrator=%v", mig.calls)
	}
	if len(journal.deleted) != 1 || journal.deleted[0] != "resize-1" {
		t.Fatalf("deleted=%v", journal.deleted)
	}
	for _, call := range api.calls {
		if call == "delete:vol-source" {
			t.Fatal("source volume must never be auto-deleted")
		}
	}
}

func TestReplacementPersistsCreatedVolumeBeforeReadinessWait(t *testing.T) {
	api := &fakeAPI{fail: "available:vol-target"}
	mig := &fakeMigrator{}
	journal := &fakeJournal{}
	op, err := New(api, mig, journal).Replace(context.Background(), spec())
	if !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("op=%#v err=%v", op, err)
	}
	if op.TargetVolumeID != "vol-target" || op.Phase != PhaseRecoveryRequired || op.RecoveryFrom != PhaseTargetCreated {
		t.Fatalf("created volume identity lost: op=%#v", op)
	}
	if len(journal.saved) < 3 {
		t.Fatalf("journal=%#v", journal.saved)
	}
	created := journal.saved[1]
	if created.Phase != PhaseTargetCreated || created.TargetVolumeID != "vol-target" {
		t.Fatalf("target identity was not durable before wait: %#v", created)
	}
	joined := strings.Join(api.calls, "|")
	if strings.Contains(joined, "attach:vol-target") || strings.Contains(joined, "delete:vol-target") {
		t.Fatalf("ambiguous target was mutated after waiter failure: %v", api.calls)
	}
	if len(api.createTokens) != 1 || api.createTokens[0] != clientTokenForOperation(spec().OperationID) {
		t.Fatalf("create tokens=%v", api.createTokens)
	}
}

func TestReplacementClientTokenIsStablePerOperation(t *testing.T) {
	first := clientTokenForOperation("resize-1")
	second := clientTokenForOperation("resize-1")
	other := clientTokenForOperation("resize-2")
	if first != second {
		t.Fatalf("client token changed across retry: %q != %q", first, second)
	}
	if first == other {
		t.Fatalf("different operations share a client token: %q", first)
	}
	if len(first) != 64 {
		t.Fatalf("client token length=%d token=%q", len(first), first)
	}
}

func TestTargetCreatedJournalFailureCleansKnownVolumeWithoutDetach(t *testing.T) {
	api := &fakeAPI{}
	mig := &fakeMigrator{}
	journal := &fakeJournal{failPhase: PhaseTargetCreated}
	op, err := New(api, mig, journal).Replace(context.Background(), spec())
	if err == nil {
		t.Fatal("expected journal failure")
	}
	if op.TargetVolumeID != "vol-target" {
		t.Fatalf("created volume identity lost: %#v", op)
	}
	joined := strings.Join(api.calls, "|")
	if !strings.Contains(joined, "available:vol-target|delete:vol-target") {
		t.Fatalf("known unattached target was not cleaned: %v", api.calls)
	}
	if strings.Contains(joined, "detach:vol-target") {
		t.Fatalf("unattached target was detached during cleanup: %v", api.calls)
	}
}

func TestReplacementFailsBeforeMutationWhenPreflightOrTargetSizeUnsafe(t *testing.T) {
	api := &fakeAPI{}
	mig := &fakeMigrator{fail: "preflight"}
	journal := &fakeJournal{}
	if _, err := New(api, mig, journal).Replace(context.Background(), spec()); err == nil {
		t.Fatal("preflight failure accepted")
	}
	if strings.Contains(strings.Join(api.calls, "|"), "create") {
		t.Fatalf("mutated before preflight: %v", api.calls)
	}

	unsafe := spec()
	unsafe.TargetSizeGiB = 100
	api = &fakeAPI{}
	mig = &fakeMigrator{}
	journal = &fakeJournal{}
	if _, err := New(api, mig, journal).Replace(context.Background(), unsafe); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v", err)
	}
	if len(api.calls) != 1 || api.calls[0] != "describe:vol-source" {
		t.Fatalf("calls=%v", api.calls)
	}
}

func TestVerificationFailureCleansOnlyReplacementVolume(t *testing.T) {
	api := &fakeAPI{}
	mig := &fakeMigrator{fail: "verify"}
	journal := &fakeJournal{}
	_, err := New(api, mig, journal).Replace(context.Background(), spec())
	if err == nil {
		t.Fatal("expected error")
	}
	joined := strings.Join(api.calls, "|")
	if !strings.Contains(joined, "delete:vol-target") || strings.Contains(joined, "detach:vol-source") {
		t.Fatalf("unsafe cleanup=%v", api.calls)
	}
	if len(journal.deleted) == 0 {
		t.Fatal("pre-detach journal not cleared")
	}
}

func TestCleanupDetachFailureRequiresRecoveryAndKeepsJournal(t *testing.T) {
	api := &fakeAPI{fail: "detach:vol-target"}
	mig := &fakeMigrator{fail: "verify"}
	journal := &fakeJournal{}
	op, err := New(api, mig, journal).Replace(context.Background(), spec())
	if !errors.Is(err, core.ErrRecoveryRequired) || op.Phase != PhaseRecoveryRequired {
		t.Fatalf("op=%#v err=%v", op, err)
	}
	joined := strings.Join(api.calls, "|")
	if strings.Contains(joined, "delete:vol-target") {
		t.Fatalf("target deleted after ambiguous detach: %v", api.calls)
	}
	if len(journal.deleted) != 0 {
		t.Fatalf("journal deleted despite failed cleanup: %v", journal.deleted)
	}
	if len(journal.saved) == 0 || journal.saved[len(journal.saved)-1].Phase != PhaseRecoveryRequired {
		t.Fatalf("journal=%#v", journal.saved)
	}
}

func TestFailureAfterSourceDetachIsRecoveryRequiredAndNeverDeletesVolumes(t *testing.T) {
	api := &fakeAPI{fail: "detach:vol-target"}
	mig := &fakeMigrator{}
	journal := &fakeJournal{}
	op, err := New(api, mig, journal).Replace(context.Background(), spec())
	if !errors.Is(err, core.ErrRecoveryRequired) || op.Phase != PhaseRecoveryRequired {
		t.Fatalf("op=%#v err=%v", op, err)
	}
	joined := strings.Join(api.calls, "|")
	if strings.Contains(joined, "delete:vol-target") || strings.Contains(joined, "delete:vol-source") {
		t.Fatalf("destructive rollback after source detach: %v", api.calls)
	}
	if len(journal.saved) == 0 || journal.saved[len(journal.saved)-1].Phase != PhaseRecoveryRequired {
		t.Fatalf("journal=%#v", journal.saved)
	}
}

func TestSourceDetachMutationErrorRequiresRecoveryInsteadOfRollback(t *testing.T) {
	api := &fakeAPI{fail: "detach:vol-source"}
	mig := &fakeMigrator{}
	journal := &fakeJournal{}
	op, err := New(api, mig, journal).Replace(context.Background(), spec())
	if !errors.Is(err, core.ErrRecoveryRequired) || op.RecoveryFrom != PhaseVerified {
		t.Fatalf("op=%#v err=%v", op, err)
	}
	joined := strings.Join(api.calls, "|")
	if strings.Contains(joined, "delete:vol-target") {
		t.Fatalf("ambiguous source detach triggered destructive rollback: %v", api.calls)
	}
}

func TestJournalFailureAfterSourceDetachAlsoRequiresRecovery(t *testing.T) {
	api := &fakeAPI{}
	mig := &fakeMigrator{}
	journal := &fakeJournal{failPhase: PhaseSourceDetached}
	op, err := New(api, mig, journal).Replace(context.Background(), spec())
	if !errors.Is(err, core.ErrRecoveryRequired) || op.Phase != PhaseRecoveryRequired || op.RecoveryFrom != PhaseSourceDetached {
		t.Fatalf("op=%#v err=%v", op, err)
	}
}

func TestReplacementRejectsConflictingPersistedOwner(t *testing.T) {
	conflictSpec := spec()
	conflictSpec.OperationID = "resize-other"
	journal := &fakeJournal{current: map[string]Operation{
		"resize-other": operationFromSpec(conflictSpec, PhaseVerified),
	}}
	api := &fakeAPI{}
	op, err := New(api, &fakeMigrator{}, journal).Replace(context.Background(), spec())
	if !errors.Is(err, core.ErrStorageBusy) || op.ID != "resize-other" {
		t.Fatalf("op=%#v err=%v", op, err)
	}
	if len(api.calls) != 0 {
		t.Fatalf("AWS mutated despite conflicting owner: %v", api.calls)
	}
}

func TestReplacementPropagatesTransactionLockContention(t *testing.T) {
	journal := &fakeJournal{lockErr: core.ErrStorageBusy}
	_, err := New(&fakeAPI{}, &fakeMigrator{}, journal).Replace(context.Background(), spec())
	if !errors.Is(err, core.ErrStorageBusy) {
		t.Fatalf("err=%v", err)
	}
}

func TestFileJournalPersistsPrivateAtomicState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "ops")
	journal := NewFileJournal(root)
	op := operationFromSpec(spec(), PhaseVerified)
	if err := journal.Save(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, op.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	if rootInfo.Mode().Perm() != 0o700 {
		t.Fatalf("root mode=%o", rootInfo.Mode().Perm())
	}
	loaded, found, err := journal.Load(context.Background(), op.ID)
	if err != nil || !found || loaded != op {
		t.Fatalf("loaded=%#v found=%v err=%v", loaded, found, err)
	}
	listed, err := journal.List(context.Background())
	if err != nil || len(listed) != 1 || listed[0] != op {
		t.Fatalf("listed=%#v err=%v", listed, err)
	}
	matches, err := filepath.Glob(filepath.Join(root, "."+op.ID+".*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary journal files=%v err=%v", matches, err)
	}
	if err := journal.Delete(context.Background(), op.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, op.ID+".json")); !os.IsNotExist(err) {
		t.Fatalf("journal remains: %v", err)
	}
}

func operationFromSpec(s ReplacementSpec, phase Phase) Operation {
	return Operation{
		Version:        operationVersion,
		ID:             s.OperationID,
		InstanceID:     s.InstanceID,
		SourceVolumeID: s.SourceVolumeID,
		TargetVolumeID: "vol-target",
		SourceDevice:   s.SourceDevice,
		StagingDevice:  s.StagingDevice,
		TargetSizeGiB:  s.TargetSizeGiB,
		Phase:          phase,
	}
}
