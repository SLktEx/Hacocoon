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

type backend struct {
	store  *state.EnvironmentJSONStore
	fail   string
	exists bool
	calls  int
}

func (b *backend) Plan(context.Context, string, string) (string, error) {
	return "pool/owned-volume", nil
}
func (b *backend) Create(ctx context.Context, r core.PersistentResource) error {
	b.calls++
	saved, err := b.store.GetPersistentResource(ctx, r.ID)
	if err != nil || saved != r {
		return errors.New("provider called before durable ownership")
	}
	b.exists = true
	if b.fail == "create" {
		return errors.New("ambiguous create")
	}
	return nil
}
func (b *backend) Verify(context.Context, core.PersistentResource) error {
	if b.fail == "verify" {
		return errors.New("verification failed")
	}
	return nil
}
func (b *backend) Delete(context.Context, core.PersistentResource) error {
	if b.fail == "delete" {
		return errors.New("uncertain deletion")
	}
	b.exists = false
	return nil
}
func TestDurableOwnershipAndExplicitDeletion(t *testing.T) {
	ctx := context.Background()
	for _, fail := range []string{"", "create", "verify", "delete"} {
		t.Run(fail, func(t *testing.T) {
			store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
			b := &backend{store: store, fail: fail}
			s := &persistentresource.Service{Store: store, Backend: b}
			r, err := s.Create(ctx, "oci:demo", "oci-containerd")
			if fail == "create" || fail == "verify" {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatalf("error=%v", err)
				}
				held, e := store.GetPersistentResource(ctx, r.ID)
				if e != nil || held.State != "creating" {
					t.Fatalf("lost reservation: %+v %v", held, e)
				}
			} else if err != nil || r.State != "ready" {
				t.Fatalf("create=%+v %v", r, err)
			}
			if _, err := s.Create(ctx, r.ID, r.Kind); !errors.Is(err, core.ErrAlreadyExists) {
				t.Fatalf("duplicate=%v", err)
			}
			if b.calls != 1 {
				t.Fatalf("duplicate reached provider: %d", b.calls)
			}
			err = s.Delete(ctx, r.ID)
			if fail == "delete" {
				held, e := store.GetPersistentResource(ctx, r.ID)
				if !errors.Is(err, core.ErrRecoveryRequired) || e != nil || held.State != "deleting" || !b.exists {
					t.Fatalf("unsafe cleanup: %+v %v %v", held, err, e)
				}
				b.fail = ""
				err = s.Delete(ctx, r.ID)
			}
			if err != nil || b.exists {
				t.Fatalf("delete=%v exists=%v", err, b.exists)
			}
			if _, err := store.GetPersistentResource(ctx, r.ID); !errors.Is(err, core.ErrNotFound) {
				t.Fatal(err)
			}
		})
	}
}

func (b *backend) CheckDeletion(context.Context, core.PersistentResource) error {
	if b.fail == "saved" {
		return core.ErrStorageBusy
	}
	return nil
}
func TestReviewedStoreDeletionPreservesReadyOnPreflightRefusal(t *testing.T) {
	ctx := context.Background()
	store := state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "state.json"))
	b := &backend{store: store}
	s := &persistentresource.Service{Store: store, Backend: b}
	r, err := s.Create(ctx, "oci:review", "oci-containerd")
	if err != nil {
		t.Fatal(err)
	}
	b.fail = "saved"
	if err = s.DeleteReviewed(ctx, r.Ref()); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal(err)
	}
	held, err := store.GetPersistentResource(ctx, r.ID)
	if err != nil || held.State != "ready" || !b.exists {
		t.Fatal(held, err)
	}
	stale := r.Ref()
	stale.Owner = "cccccccccccccccccccccccccccccccc"
	if err = s.DeleteReviewed(ctx, stale); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal(err)
	}
	b.fail = "delete"
	if err = s.DeleteReviewed(ctx, r.Ref()); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
	held, err = store.GetPersistentResource(ctx, r.ID)
	if err != nil || held.State != "deleting" || held.Ref() != r.Ref() {
		t.Fatal(held, err)
	}
	b.fail = ""
	if err = s.DeleteReviewed(ctx, r.Ref()); err != nil || b.exists {
		t.Fatal(err)
	}
}
