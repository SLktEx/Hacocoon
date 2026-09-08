package persistentresource_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
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
				if _, err := svc.RecoverCopy(ctx, target.ID); !errors.Is(err, core.ErrRecoveryRequired) || b.recoveries != 0 {
					t.Fatal("unknown completion resumed")
				}
				return
			}
			b.rejectRecovery = true
			if _, err := svc.RecoverCopy(ctx, target.ID); !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal(err)
			}
			held, _ := b.store.GetPersistentResource(ctx, target.ID)
			if !held.CopyCompleted || held.State != "creating" {
				t.Fatal("failed recovery released reservation")
			}
			b.rejectRecovery = false
			recovered, err := svc.RecoverCopy(ctx, target.ID)
			if err != nil || recovered.State != "ready" || recovered.CopyCompleted || recovered.CopySource != (core.PersistentResourceRef{}) || recovered.Owner != target.Owner || recovered.WorkspaceID != "work" || b.copies != 1 {
				t.Fatalf("recovery: %+v %v copies=%d", recovered, err, b.copies)
			}
			b.fail = "verify" // An already published copy may now be attached.
			if again, err := svc.RecoverCopy(ctx, target.ID); err != nil || again != recovered {
				t.Fatal("retry not idempotent")
			}
			b.fail = ""
			if err := svc.Delete(ctx, source.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}
