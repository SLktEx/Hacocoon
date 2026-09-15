package workspace

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type inspectSnapshotRuntime struct {
	*captureRuntime
	observe func(context.Context, core.SnapshotComponent) (core.SnapshotComponentInspection, error)
}

func (r *inspectSnapshotRuntime) InspectSnapshotComponent(ctx context.Context, c core.SnapshotComponent) (core.SnapshotComponentInspection, error) {
	return r.observe(ctx, c)
}

func TestInspectPartialDeletionRetainsOwnershipAndSafeRetry(t *testing.T) {
	svc, store, backend := captureFixture(t)
	ctx := context.Background()
	saved, err := svc.CaptureSnapshot(ctx, "resume")
	if err != nil {
		t.Fatal(err)
	}
	store.trace.fail = "delete:workspace:main"
	if err := svc.DeleteSnapshot(ctx, saved.ID); err == nil || !strings.Contains(err.Error(), `component "workspace:main"`) {
		t.Fatal("missing failed component", err)
	}
	before, err := store.GetSnapshot(ctx, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	calls := []string{}
	svc.runtime = &inspectSnapshotRuntime{captureRuntime: backend, observe: func(probe context.Context, c core.SnapshotComponent) (core.SnapshotComponentInspection, error) {
		calls = append(calls, c.Role)
		if _, ok := probe.Deadline(); !ok {
			t.Fatal("unbounded observation")
		}
		locked, cancel := context.WithTimeout(probe, 20*time.Millisecond)
		defer cancel()
		if err := svc.DeleteSnapshot(locked, saved.ID); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("inspection did not hold deletion locks", err)
		}
		if c.Role == "rootfs" {
			return core.SnapshotComponentInspection{Presence: "absent", Check: "absent"}, nil
		}
		n := 1
		return core.SnapshotComponentInspection{Presence: "present", Check: "busy", References: &n}, nil
	}}
	got, err := svc.InspectSnapshot(ctx, saved.ID)
	if err != nil || got.Partial || got.State != "deleting" || len(got.Components) != 2 || !reflect.DeepEqual(calls, []string{"rootfs", "workspace:main"}) {
		t.Fatal(got, calls, err)
	}
	if got.Components[0].State != "absent" || got.Components[1].Check != "busy" {
		t.Fatal(got)
	}
	after, err := store.GetSnapshot(ctx, saved.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("inspection mutated ownership", after, err)
	}
	store.trace.fail = ""
	store.trace.events = nil
	if err := svc.DeleteSnapshot(ctx, saved.ID); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.Join(store.trace.events, ","), "delete:rootfs") {
		t.Fatal("retry re-deleted absent component")
	}
	if _, err := store.GetSnapshot(ctx, saved.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestInspectFailureKeepsIndependentObservations(t *testing.T) {
	svc, store, backend := captureFixture(t)
	ctx := context.Background()
	saved, err := svc.CaptureSnapshot(ctx, "resume")
	if err != nil {
		t.Fatal(err)
	}
	svc.runtime = &inspectSnapshotRuntime{captureRuntime: backend, observe: func(_ context.Context, c core.SnapshotComponent) (core.SnapshotComponentInspection, error) {
		if c.Role == "rootfs" {
			return core.SnapshotComponentInspection{}, errors.New("synthetic-secret provider response")
		}
		return core.SnapshotComponentInspection{Presence: "present", Check: "ready"}, nil
	}}
	got, err := svc.InspectSnapshot(ctx, saved.ID)
	if !errors.Is(err, core.ErrRecoveryRequired) || !got.Partial || len(got.Components) != 2 || got.Components[0].Check != "unavailable" || got.Components[0].Presence != "unknown" || got.Components[1].Check != "ready" {
		t.Fatal(got, err)
	}
	after, err := store.GetSnapshot(ctx, saved.ID)
	if err != nil || !reflect.DeepEqual(saved, after) {
		t.Fatal("ownership changed", after, err)
	}
}
