package persistentresource_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
)

type stagedCopyBackend struct {
	*copyBackend
	recoveries     int
	rejectRecovery bool
}

func (b *stagedCopyBackend) CopyWithCompletion(ctx context.Context, source, target core.PersistentResource, completed func() error) error {
	if err := b.Copy(ctx, source, target); err != nil {
		return err
	}
	if err := completed(); err != nil {
		return err
	}
	saved, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || !saved.CopyCompleted || saved.State != "creating" || saved.CopySource != source.Ref() {
		b.test.Fatal("restored source before durable completion")
	}
	if _, err := b.store.BeginPersistentResourceDelete(ctx, source.ID); !errors.Is(err, core.ErrStorageBusy) {
		b.test.Fatal("completion released source too early")
	}
	return errors.New("source resume interrupted")
}
func (b *stagedCopyBackend) RecoverCompletedCopy(ctx context.Context, source, target core.PersistentResource) error {
	b.recoveries++
	saved, err := b.store.GetPersistentResource(ctx, target.ID)
	if err != nil || saved != target || !target.CopyCompleted || target.CopySource != source.Ref() {
		b.test.Fatal("recovery without durable exact receipt")
	}
	if b.rejectRecovery {
		return core.ErrRecoveryRequired
	}
	return nil
}
func TestCompletedCopyRecoversAfterReopeningStateWithoutRecopying(t *testing.T) {
	for _, mode := range []string{"completed", "unconfirmed", "verification-failed"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "state.json")
			st := state.NewEnvironmentJSONStore(path)
			b := &stagedCopyBackend{copyBackend: &copyBackend{backend: backend{store: st}, test: t}}
			svc := &persistentresource.Service{Store: st, Backend: b}
			source, err := svc.Create(ctx, "oci:source", "oci-containerd")
			if err != nil {
				t.Fatal(err)
			}
			if mode == "unconfirmed" {
				b.fail = "copy"
			}
			if mode == "verification-failed" {
				b.fail = "verify"
			}
			target, err := svc.CopyForWorkspace(ctx, "oci:target", source.Kind, source.ID, "work")
			if !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
			b.store = state.NewEnvironmentJSONStore(path)
			svc = &persistentresource.Service{Store: b.store, Backend: b}
			b.fail = ""
			if mode != "completed" {
				if err := b.store.CommitPersistentResourceCreate(ctx, target); !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("copy published without completion receipt", err)
				}
				if _, err := svc.RecoverCopy(ctx, target.Ref()); !errors.Is(err, core.ErrRecoveryRequired) || b.recoveries != 0 {
					t.Fatal("unknown completion resumed")
				}
				return
			}
			b.rejectRecovery = true
			if _, err := svc.RecoverCopy(ctx, target.Ref()); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
			held, _ := b.store.GetPersistentResource(ctx, target.ID)
			if !held.CopyCompleted || held.State != "creating" {
				t.Fatal("failed recovery released reservation")
			}
			b.rejectRecovery = false
			recovered, err := svc.RecoverCopy(ctx, target.Ref())
			if err != nil || recovered.State != "ready" || recovered.CopyCompleted || recovered.CopySource != (core.PersistentResourceRef{}) || recovered.Owner != target.Owner || recovered.WorkspaceID != "work" || b.copies != 1 {
				t.Fatalf("recovery: %+v %v copies=%d", recovered, err, b.copies)
			}
			b.fail = "verify" // An already published copy may now be attached.
			if again, err := svc.RecoverCopy(ctx, target.Ref()); err != nil || again != recovered {
				t.Fatal("retry not idempotent")
			}
			b.fail = ""
			if err := svc.Delete(ctx, source.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// Fail on either side of the real durable commit. Recovery must distinguish a
// lost reply from a failed write without repeating the native copy.
type recoveryCommitFault struct {
	*state.EnvironmentJSONStore
	afterWrite bool
	readFails  bool
	committed  bool
	err        error
}

func (s *recoveryCommitFault) CommitPersistentResourceCreate(ctx context.Context, r core.PersistentResource) error {
	if s.afterWrite {
		if err := s.EnvironmentJSONStore.CommitPersistentResourceCreate(ctx, r); err != nil {
			return err
		}
	}
	s.committed = true
	return s.err
}

func (s *recoveryCommitFault) GetPersistentResource(ctx context.Context, id string) (core.PersistentResource, error) {
	if s.committed && s.readFails {
		return core.PersistentResource{}, s.err
	}
	return s.EnvironmentJSONStore.GetPersistentResource(ctx, id)
}

func TestCopyRecoveryResolvesCommitUncertaintyFromDurableIdentity(t *testing.T) {
	for _, mode := range []string{"before-write", "after-write", "after-write-read-failed"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "state.json")
			st := state.NewEnvironmentJSONStore(path)
			b := &stagedCopyBackend{copyBackend: &copyBackend{backend: backend{store: st}, test: t}}
			svc := &persistentresource.Service{Store: st, Backend: b}
			source, err := svc.Create(ctx, "oci:source", "oci-containerd")
			if err != nil {
				t.Fatal(err)
			}
			target, err := svc.CopyForWorkspace(ctx, "oci:target", source.Kind, source.ID, "work")
			if !errors.Is(err, core.ErrRecoveryRequired) || !target.CopyCompleted {
				t.Fatal("fixture did not retain a completed copy", target, err)
			}
			fault := &recoveryCommitFault{EnvironmentJSONStore: state.NewEnvironmentJSONStore(path), afterWrite: mode != "before-write", readFails: mode == "after-write-read-failed", err: errors.New("catalog reply lost")}
			svc.Store = fault
			result, err := svc.RecoverCopy(ctx, target.Ref())
			if mode == "after-write" {
				if err != nil || result.State != "ready" || result.Ref() != target.Ref() {
					t.Fatal("durably committed identity was not recognized", result, err)
				}
			} else if !errors.Is(err, fault.err) || result != target {
				t.Fatal("uncertain publication was reported as successful", result, err)
			}
			reopened := state.NewEnvironmentJSONStore(path)
			held, err := reopened.GetPersistentResource(ctx, target.ID)
			if err != nil || held.Ref() != target.Ref() || held.WorkspaceID != "work" {
				t.Fatal("recovery lost exact ownership", held, err)
			}
			if mode == "before-write" {
				if held != target {
					t.Fatal("failed commit lost the completion receipt", held)
				}
				if _, err := reopened.BeginPersistentResourceDelete(ctx, source.ID); !errors.Is(err, core.ErrStorageBusy) {
					t.Fatal("failed commit released the copy source", err)
				}
			} else if held.State != "ready" || held.CopyCompleted || held.CopySource != (core.PersistentResourceRef{}) {
				t.Fatal("successful commit retained an unfinished copy", held)
			}
			svc.Store = reopened
			recovered, err := svc.RecoverCopy(ctx, target.Ref())
			wantRecoveries := 1
			if mode == "before-write" {
				wantRecoveries++
			}
			if err != nil || recovered.Ref() != target.Ref() || recovered.State != "ready" || recovered.WorkspaceID != "work" || b.copies != 1 || b.recoveries != wantRecoveries {
				t.Fatal("retry copied again or changed ownership", recovered, err, b.copies, b.recoveries)
			}
			if err := svc.Delete(ctx, source.ID); err != nil {
				t.Fatal("successful recovery did not release the source", err)
			}
		})
	}
}
