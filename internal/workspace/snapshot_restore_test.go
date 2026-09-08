package workspace

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type restoreTraceStore struct {
	*state.EnvironmentJSONStore
	fail string
}

func (s *restoreTraceStore) step(name string) error {
	if s.fail == name {
		return errors.New("injected " + name)
	}
	return nil
}
func (s *restoreTraceStore) BeginSnapshotRestore(ctx context.Context, op core.SnapshotRestore) error {
	if err := s.step("reserve"); err != nil {
		return err
	}
	return s.EnvironmentJSONStore.BeginSnapshotRestore(ctx, op)
}
func (s *restoreTraceStore) RecordRestoreComponent(ctx context.Context, id string, c core.SnapshotComponent, next string) error {
	if err := s.step("record-" + next + ":" + c.Role); err != nil {
		return err
	}
	return s.EnvironmentJSONStore.RecordRestoreComponent(ctx, id, c, next)
}
func (s *restoreTraceStore) CommitRestorePreparation(ctx context.Context, id string) error {
	if err := s.step("commit"); err != nil {
		return err
	}
	return s.EnvironmentJSONStore.CommitRestorePreparation(ctx, id)
}

type restoreTraceRuntime struct {
	*snapshotRuntime
	t           *testing.T
	store       *restoreTraceStore
	nextOwner   int
	plans       map[string]string
	data        map[string]string
	current     string
	savedID     string
	restoreID   string
	fail        string
	cleanupFail string
	cancel      context.CancelFunc
}

func (r *restoreTraceRuntime) components(id string) []core.SnapshotComponent {
	out := []core.SnapshotComponent{}
	for _, role := range []string{"rootfs", "workspace:main"} {
		r.nextOwner++
		out = append(out, core.SnapshotComponent{Role: role, NativeRef: id + "/" + role, Owner: fmt.Sprintf("%032x", r.nextOwner), Binding: `{"fixture":1}`, State: "planned"})
	}
	return out
}
func (r *restoreTraceRuntime) PlanSnapshot(_ context.Context, _ core.SnapshotSource, id string) ([]core.SnapshotComponent, error) {
	components := r.components(id)
	for _, c := range components {
		r.plans[c.NativeRef] = id
	}
	return components, nil
}
func (r *restoreTraceRuntime) CreateSnapshotComponent(ctx context.Context, _ core.SnapshotSource, c core.SnapshotComponent) error {
	if r.fail == "backup" {
		return errors.New("injected backup")
	}
	snap, err := r.store.GetSnapshot(ctx, r.plans[c.NativeRef])
	if err != nil || snap.State != "capturing" {
		r.t.Fatal("backup create without receipt", err)
	}
	r.data[c.NativeRef] = r.current + ":" + c.Role
	return nil
}
func (r *restoreTraceRuntime) VerifySnapshotComponent(ctx context.Context, c core.SnapshotComponent) error {
	if r.fail == "source" && r.plans[c.NativeRef] == r.savedID {
		return errors.New("injected source")
	}
	snap, err := r.store.GetSnapshot(ctx, r.plans[c.NativeRef])
	if err != nil {
		r.t.Fatal(err)
	}
	for _, stored := range snap.Components {
		if stored.NativeRef == c.NativeRef && (stored.State == "created" || stored.State == "verified") && r.data[c.NativeRef] != "" {
			return nil
		}
	}
	r.t.Fatal("snapshot verification before durable create", c)
	return nil
}
func (r *restoreTraceRuntime) DeleteSnapshotComponent(_ context.Context, c core.SnapshotComponent) error {
	delete(r.data, c.NativeRef)
	return nil
}
func (r *restoreTraceRuntime) PlanSnapshotRestore(_ context.Context, saved core.Snapshot, id string) ([]core.SnapshotComponent, error) {
	if r.fail == "plan" {
		return nil, errors.New("injected plan")
	}
	if saved.ID != r.savedID {
		r.t.Fatal("wrong source")
	}
	r.restoreID = id
	return r.components(id), nil
}
func (r *restoreTraceRuntime) receipt(c core.SnapshotComponent, state string) core.SnapshotRestore {
	r.t.Helper()
	op, err := r.store.GetSnapshotRestore(context.Background(), r.restoreID)
	if err != nil {
		r.t.Fatal("restore operation missing", err)
	}
	for _, got := range op.Components {
		if got.Role == c.Role {
			expected := c
			expected.State = state
			if got != expected {
				r.t.Fatal("wrong restore receipt", got, expected)
			}
			return op
		}
	}
	r.t.Fatal("restore target missing")
	return op
}
func (r *restoreTraceRuntime) CreateRestoreComponent(_ context.Context, saved core.Snapshot, c core.SnapshotComponent) error {
	op := r.receipt(c, "planned")
	if op.Before.ID != "" {
		r.t.Fatal("unexpected automatic backup")
	}
	for _, src := range saved.Components {
		if src.Role == c.Role {
			r.data[c.NativeRef] = r.data[src.NativeRef]
		}
	}
	if r.cancel != nil {
		r.cancel()
	}
	if r.fail == "create:"+c.Role {
		return errors.New("injected lost create reply")
	}
	return nil
}
func (r *restoreTraceRuntime) VerifyRestoreComponent(_ context.Context, c core.SnapshotComponent) error {
	r.receipt(c, "created")
	if r.fail == "verify:"+c.Role {
		return errors.New("injected verification")
	}
	return nil
}
func (r *restoreTraceRuntime) DeleteRestoreComponent(_ context.Context, c core.SnapshotComponent) error {
	op := r.receipt(c, c.State)
	if op.State != "deleting" {
		r.t.Fatal("cleanup not durable")
	}
	if r.fail == "delete:"+c.Role || r.cleanupFail == "delete:"+c.Role {
		return errors.New("injected cleanup")
	}
	delete(r.data, c.NativeRef)
	return nil
}

func TestRestoreServicePreservesCurrentWorkAndEveryPartialFailure(t *testing.T) {
	for _, failure := range []string{"", "source", "plan", "reserve", "create:rootfs", "record-created:rootfs", "verify:workspace:main", "record-verified:workspace:main", "commit", "cancel", "create-and-cleanup", "commit-and-root-cleanup"} {
		t.Run(failure, func(t *testing.T) {
			_, catalog, original := captureFixture(t)
			store := &restoreTraceStore{EnvironmentJSONStore: catalog.EnvironmentJSONStore}
			r := &restoreTraceRuntime{snapshotRuntime: original.snapshotRuntime, t: t, store: store, plans: map[string]string{}, data: map[string]string{}, current: "saved"}
			svc := New(r, store)
			ctx := context.Background()
			saved, err := svc.CaptureSnapshot(ctx, "resume")
			if err != nil {
				t.Fatal(err)
			}
			r.savedID = saved.ID
			r.current = "new uncommitted work"
			expectedEnv, err := store.GetEnvironment(ctx, "resume")
			if err != nil {
				t.Fatal(err)
			}
			r.fail = failure
			store.fail = failure
			if failure == "create-and-cleanup" {
				r.fail = "create:rootfs"
				r.cleanupFail = "delete:workspace:main"
			}
			if failure == "commit-and-root-cleanup" {
				store.fail = "commit"
				r.cleanupFail = "delete:rootfs"
			}
			request, cancel := context.WithCancel(ctx)
			defer cancel()
			if failure == "cancel" {
				r.cancel = cancel
			}
			op, err := svc.PrepareSnapshotRestore(request, "resume", saved.ID)
			if failure == "" {
				if err != nil || op.State != "prepared" {
					t.Fatal(op, err)
				}
			} else if err == nil {
				t.Fatal("injected failure succeeded")
			}
			if len(r.plans) != len(saved.Components) {
				t.Fatal("restore created an automatic snapshot")
			}
			current, readErr := store.GetEnvironment(ctx, "resume")
			if readErr != nil || !reflect.DeepEqual(current, expectedEnv) {
				t.Fatal("current Environment changed", readErr)
			}
			for _, c := range saved.Components {
				if r.data[c.NativeRef] != "saved:"+c.Role {
					t.Fatal("saved data changed")
				}
			}
			if op.ID == "" {
				if len(r.data) != len(saved.Components) {
					t.Fatal("temporary resources leaked after completed cleanup")
				}
				if e := store.CheckSnapshotIdle(ctx, "resume"); e != nil {
					t.Fatal("clean failure blocked current Environment", e)
				}
				return
			}
			reloaded := state.NewEnvironmentJSONStore(catalog.path)
			persisted, e := reloaded.GetSnapshotRestore(ctx, op.ID)
			if e != nil {
				t.Fatal(e)
			}
			if failure != "" && !errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal("failure lost recovery indication", err)
			}
			if reloaded.BeginSnapshotDelete(ctx, saved.ID) == nil || reloaded.CheckSnapshotIdle(ctx, "resume") == nil {
				t.Fatal("restore references released")
			}
			if failure == "commit-and-root-cleanup" {
				if persisted.Components[0].State == "absent" || persisted.Components[1].State != "absent" {
					t.Fatal("one cleanup failure blocked independent cleanup")
				}
			}
			store.fail = ""
			r.cancel = nil
			r.fail = "delete:workspace:main"
			if svc.CleanupSnapshotRestore(ctx, op.ID) == nil {
				t.Fatal("failed cleanup succeeded")
			}
			if _, e := reloaded.GetSnapshotRestore(ctx, op.ID); e != nil {
				t.Fatal("partial cleanup lost ownership", e)
			}
			r.fail = ""
			r.cleanupFail = ""
			if e := svc.CleanupSnapshotRestore(ctx, op.ID); e != nil {
				t.Fatal(e)
			}
			for _, c := range persisted.Components {
				if _, exists := r.data[c.NativeRef]; exists {
					t.Fatal("copy remains")
				}
			}
			if e := reloaded.CheckSnapshotIdle(ctx, "resume"); e != nil {
				t.Fatal(e)
			}
		})
	}
}
