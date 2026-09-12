package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type readSnapshotRuntime struct {
	*captureRuntime
	verified []string
	failure  error
}

func (r *readSnapshotRuntime) VerifySnapshotComponent(_ context.Context, c core.SnapshotComponent) error {
	r.verified = append(r.verified, c.Role)
	if c.State != "verified" {
		return core.ErrIncompatibleState
	}
	return r.failure
}
func readSnapshotFixture(t *testing.T) (*Service, *captureStore, *readSnapshotRuntime, core.Snapshot) {
	t.Helper()
	svc, store, backend := captureFixture(t)
	saved, err := svc.CaptureSnapshot(context.Background(), "resume")
	if err != nil {
		t.Fatal(err)
	}
	runtime := &readSnapshotRuntime{captureRuntime: backend}
	svc.runtime = runtime
	return svc, store, runtime, saved
}
func TestSnapshotReadBlocksDeleteThroughConsumption(t *testing.T) {
	svc, store, runtime, saved := readSnapshotFixture(t)
	ctx := context.Background()
	// Separate service/catalog objects exercise the canonical shared locks.
	other := New(runtime, state.NewEnvironmentJSONStore(store.path))
	err := svc.ReadSnapshot(ctx, saved.ID, func(ctx context.Context, got core.Snapshot) error {
		if !reflect.DeepEqual(runtime.verified, []string{"rootfs", "workspace:main"}) {
			t.Fatal("consumed before all ownership checks", runtime.verified)
		}
		if !reflect.DeepEqual(got, saved) {
			t.Fatal("changed saved metadata")
		}
		limited, cancel := context.WithTimeout(ctx, 60*time.Millisecond)
		defer cancel()
		if err := other.DeleteSnapshot(limited, saved.ID); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("delete bypassed reader", err)
		}
		current, err := store.GetSnapshot(ctx, saved.ID)
		if err != nil || current.State != "ready" {
			t.Fatal("blocked delete changed catalog", current, err)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := other.DeleteSnapshot(ctx, saved.ID); err != nil {
		t.Fatal("reader did not release locks", err)
	}
	if _, err := store.GetSnapshot(ctx, saved.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}
func TestSnapshotReadFailureNeverDeletesSavedData(t *testing.T) {
	for _, failure := range []string{"verification", "consumer", "canceled", "deleting"} {
		t.Run(failure, func(t *testing.T) {
			svc, store, runtime, saved := readSnapshotFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			injected := errors.New("injected failure")
			called := false
			switch failure {
			case "verification":
				runtime.failure = injected
			case "canceled":
				cancel()
			case "deleting":
				if err := store.BeginSnapshotDelete(context.Background(), saved.ID); err != nil {
					t.Fatal(err)
				}
			}
			err := svc.ReadSnapshot(ctx, saved.ID, func(context.Context, core.Snapshot) error { called = true; return injected })
			if err == nil || called != (failure == "consumer") {
				t.Fatal(called, err)
			}
			current, err := store.GetSnapshot(context.Background(), saved.ID)
			if err != nil || !reflect.DeepEqual(current.Components, saved.Components) {
				t.Fatal("read failure changed saved data", current, err)
			}
			// Failure leaves neither an export reservation nor a held lifecycle lock.
			runtime.failure = nil
			if failure != "deleting" {
				if err := svc.ReadSnapshot(context.Background(), saved.ID, func(context.Context, core.Snapshot) error { return nil }); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestSnapshotReadAfterSourceEnvironmentDeletion(t *testing.T) {
	svc, store, _, saved := readSnapshotFixture(t)
	ctx := context.Background()
	if err := svc.Delete(ctx, "resume"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetEnvironment(ctx, "resume"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("source Env still present", err)
	}
	if _, err := store.GetWorkspaceLease(ctx, "resume"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("source lease still present", err)
	}
	if err := svc.ReadSnapshot(ctx, saved.ID, func(_ context.Context, got core.Snapshot) error {
		if !reflect.DeepEqual(got, saved) {
			t.Fatal("saved source changed")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

type legacyReadStore struct{ *captureStore }

func (s *legacyReadStore) GetSnapshot(ctx context.Context, id string) (core.Snapshot, error) {
	saved, err := s.captureStore.GetSnapshot(ctx, id)
	if err != nil {
		return saved, err
	}
	// Model a decoded historical ready save. There is no Base provider or image cache.
	saved.Source.Environment.Base = &core.BaseRef{Name: "old", Revision: "old-revision"}
	saved.Components = append(saved.Components, core.SnapshotComponent{Role: "base", NativeRef: "missing-base", Owner: "cccccccccccccccccccccccccccccccc", Binding: "legacy-binding", State: "verified"})
	return saved, nil
}
func TestSnapshotReadDoesNotVerifyHistoricalBaseFilesystem(t *testing.T) {
	svc, store, runtime, saved := readSnapshotFixture(t)
	svc.store = &legacyReadStore{store}
	if err := svc.ReadSnapshot(context.Background(), saved.ID, func(_ context.Context, got core.Snapshot) error {
		if got.Source.Environment.Base == nil || len(got.Components) != 3 {
			t.Fatal("historical metadata discarded")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(runtime.verified, []string{"rootfs", "workspace:main"}) {
		t.Fatal("Base filesystem required", runtime.verified)
	}
}
