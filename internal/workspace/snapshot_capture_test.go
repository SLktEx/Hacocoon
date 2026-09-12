package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
)

type captureTrace struct {
	events []string
	fail   string
	cancel context.CancelFunc
}

func (f *captureTrace) step(event string) error {
	f.events = append(f.events, event)
	if f.fail == event {
		return errors.New("injected failure")
	}
	return nil
}

type captureStore struct {
	*state.EnvironmentJSONStore
	trace *captureTrace
	path  string
}

func (s *captureStore) BeginSnapshot(ctx context.Context, snap core.Snapshot) error {
	if err := s.trace.step("reserve"); err != nil {
		return err
	}
	return s.EnvironmentJSONStore.BeginSnapshot(ctx, snap)
}
func (s *captureStore) RecordSnapshotComponent(ctx context.Context, id string, c core.SnapshotComponent, next string) error {
	if err := s.trace.step("record-" + next + ":" + c.Role); err != nil {
		return err
	}
	return s.EnvironmentJSONStore.RecordSnapshotComponent(ctx, id, c, next)
}
func (s *captureStore) CommitSnapshot(ctx context.Context, id string) error {
	if err := s.trace.step("commit"); err != nil {
		return err
	}
	return s.EnvironmentJSONStore.CommitSnapshot(ctx, id)
}
func (s *captureStore) MarkSnapshotRecovery(ctx context.Context, id string) error {
	if err := s.trace.step("recovery"); err != nil {
		return err
	}
	return s.EnvironmentJSONStore.MarkSnapshotRecovery(ctx, id)
}

type captureRuntime struct {
	*snapshotRuntime
	trace *captureTrace
	store *captureStore
	id    string
	t     *testing.T
}

func (r *captureRuntime) PlanSnapshot(_ context.Context, _ core.SnapshotSource, id string) ([]core.SnapshotComponent, error) {
	r.id = id
	if err := r.trace.step("plan"); err != nil {
		return nil, err
	}
	return []core.SnapshotComponent{
		{Role: "rootfs", NativeRef: "saved-root", Owner: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", State: "planned"},
		{Role: "workspace:main", NativeRef: "saved-work", Owner: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", State: "planned"},
	}, nil
}
func (r *captureRuntime) checkReceipt(c core.SnapshotComponent, want string) {
	r.t.Helper()
	snap, err := r.store.GetSnapshot(context.Background(), r.id)
	if err != nil {
		r.t.Fatal("provider before reservation", err)
	}
	for _, got := range snap.Components {
		if got.Role == c.Role {
			if got.State != want || got.Owner != c.Owner || got.NativeRef != c.NativeRef {
				r.t.Fatal("wrong durable receipt", got)
			}
			return
		}
	}
	r.t.Fatal("missing planned target")
}
func (r *captureRuntime) CreateSnapshotComponent(_ context.Context, _ core.SnapshotSource, c core.SnapshotComponent) error {
	r.checkReceipt(c, "planned")
	if r.trace.cancel != nil {
		r.trace.cancel()
	}
	return r.trace.step("create:" + c.Role)
}
func (r *captureRuntime) VerifySnapshotComponent(_ context.Context, c core.SnapshotComponent) error {
	r.checkReceipt(c, "created")
	return r.trace.step("verify:" + c.Role)
}
func (r *captureRuntime) DeleteSnapshotComponent(_ context.Context, c core.SnapshotComponent) error {
	snap, err := r.store.GetSnapshot(context.Background(), r.id)
	if err != nil || snap.State != "deleting" {
		r.t.Fatal("deletion before durable transition", err)
	}
	return r.trace.step("delete:" + c.Role)
}
func captureFixture(t *testing.T) (*Service, *captureStore, *captureRuntime) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "state.json")
	s := &captureStore{state.NewEnvironmentJSONStore(path), &captureTrace{}, path}
	fake, rt := snapshotFixture()
	env := fake.environments["resume"]
	lease := fake.leases["resume"]
	lease.State = core.WorkspaceLeaseAcquiring
	lease.RuntimeRef = ""
	lease.AcquiredAt = time.Now().UTC()
	env.CreatedAt = lease.AcquiredAt
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.BeginEnvironmentCreate(ctx, lease))
	lease.RuntimeRef = env.RuntimeRef
	must(s.RecordEnvironmentRuntime(ctx, lease))
	lease.State = core.WorkspaceLeaseActive
	must(s.CommitEnvironmentCreate(ctx, env, lease))
	runtime := &captureRuntime{snapshotRuntime: rt, trace: s.trace, store: s, t: t}
	return New(runtime, s), s, runtime
}
func TestSnapshotCaptureOrdersDurableReceiptsAndRetainsEveryFailure(t *testing.T) {
	steps := []string{"plan", "reserve", "create:rootfs", "record-created:rootfs", "verify:rootfs", "record-verified:rootfs", "create:workspace:main", "record-created:workspace:main", "verify:workspace:main", "record-verified:workspace:main", "commit"}
	for failure := -1; failure < len(steps); failure++ {
		name := "success"
		if failure >= 0 {
			name = steps[failure]
		}
		t.Run(name, func(t *testing.T) {
			svc, s, r := captureFixture(t)
			if failure >= 0 {
				s.trace.fail = steps[failure]
			}
			snap, err := svc.CaptureSnapshot(context.Background(), "resume")
			if failure < 0 {
				if err != nil || snap.State != "ready" {
					t.Fatal(snap, err)
				}
				if !reflect.DeepEqual(s.trace.events, steps) {
					t.Fatal(s.trace.events)
				}
				return
			}
			if err == nil {
				t.Fatal("injected failure succeeded")
			}
			expected := append([]string{}, steps[:failure+1]...)
			if failure >= 2 {
				expected = append(expected, "recovery")
				if snap.ID == "" || !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("lost recovery handle", err)
				}
				reloaded := state.NewEnvironmentJSONStore(s.path)
				if !errors.Is(reloaded.CheckSnapshotIdle(context.Background(), "resume"), core.ErrRecoveryRequired) {
					t.Fatal("lost source reservation")
				}
			} else if snap.ID != "" {
				t.Fatal("unreserved handle returned")
			}
			if !reflect.DeepEqual(s.trace.events, expected) {
				t.Fatal(s.trace.events, expected)
			}
			if failure >= 2 {
				s.trace.fail = ""
				if err := svc.DeleteSnapshot(context.Background(), snap.ID); err != nil {
					t.Fatal(err)
				}
				if err := s.CheckSnapshotIdle(context.Background(), "resume"); err != nil {
					t.Fatal(err)
				}
				if r.id != snap.ID {
					t.Fatal("wrong cleanup target")
				}
			}
		})
	}
}
func TestSnapshotCaptureCancellationAndAmbiguousCleanup(t *testing.T) {
	svc, s, r := captureFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	r.trace.cancel = cancel
	snap, err := svc.CaptureSnapshot(ctx, "resume")
	if !errors.Is(err, context.Canceled) || snap.State != "recovery-required" {
		t.Fatal(snap, err)
	}
	s.trace.fail = "delete:workspace:main"
	if !errors.Is(svc.DeleteSnapshot(context.Background(), snap.ID), core.ErrRecoveryRequired) {
		t.Fatal("cleanup failure swallowed")
	}
	saved, err := s.GetSnapshot(context.Background(), snap.ID)
	if err != nil || saved.State != "deleting" || saved.Components[0].State != "absent" || saved.Components[1].State != "planned" {
		t.Fatal(saved, err)
	}
	s.trace.fail = ""
	s.trace.events = nil
	if err := svc.DeleteSnapshot(context.Background(), snap.ID); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.trace.events, []string{"delete:workspace:main", "record-absent:workspace:main"}) {
		t.Fatal("retry repeated completed deletion", s.trace.events)
	}
}

func TestSnapshotCaptureKeepsReservationWhenRecoveryWriteFails(t *testing.T) {
	svc, s, r := captureFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.trace.cancel = cancel
	s.trace.fail = "recovery"
	snap, err := svc.CaptureSnapshot(ctx, "resume")
	if !errors.Is(err, core.ErrRecoveryRequired) || snap.ID == "" {
		t.Fatal("lost reservation", err)
	}
	reopened := state.NewEnvironmentJSONStore(s.path)
	saved, err := reopened.GetSnapshot(context.Background(), snap.ID)
	if err != nil || saved.State != "capturing" || saved.Components[0].State != "planned" {
		t.Fatal(saved, err)
	}
	if !errors.Is(reopened.CheckSnapshotIdle(context.Background(), "resume"), core.ErrRecoveryRequired) {
		t.Fatal("failed marker released source")
	}
}
func TestSnapshotCaptureWithoutBackendDoesNotReserve(t *testing.T) {
	s, r := snapshotFixture()
	snapshot, err := New(r, s).CaptureSnapshot(context.Background(), "resume")
	if !errors.Is(err, core.ErrUnsupported) || snapshot.ID != "" {
		t.Fatal(snapshot, err)
	}
}
