package workspace

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type quiesceRuntime struct {
	*captureRuntime
	running bool
	events  []string
	failure string
}

func (r *quiesceRuntime) InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
	r.events = append(r.events, "inspect")
	state := core.EnvironmentStopped
	if r.running {
		state = core.EnvironmentRunning
	}
	return core.EnvironmentRuntimeStatus{State: state}, nil
}
func (r *quiesceRuntime) StopEnvironment(context.Context, string) error {
	r.events = append(r.events, "stop")
	if r.failure == "stop" {
		return errors.New("stop failed")
	}
	if r.failure != "stop-unconfirmed" {
		r.running = false
	}
	if r.failure == "identity" {
		r.identityErr = core.ErrCapabilityStale
	}
	return nil
}
func (r *quiesceRuntime) StartEnvironment(context.Context, string) error {
	r.events = append(r.events, "start")
	if r.failure == "start" {
		return errors.New("start failed")
	}
	if r.failure != "start-unconfirmed" {
		r.running = true
	}
	return nil
}
func TestSnapshotCapturePreservesSourceState(t *testing.T) {
	for _, running := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			_, store, backend := captureFixture(t)
			rt := &quiesceRuntime{captureRuntime: backend, running: running}
			svc := New(rt, store)
			if fail {
				store.trace.fail = "verify:rootfs"
			}
			saved, err := svc.CaptureSnapshot(context.Background(), "resume")
			if !fail && (err != nil || saved.State != "ready") {
				t.Fatal(saved, err)
			}
			if fail && err == nil {
				t.Fatal("capture failure lost")
			}
			if rt.running != running || !reflect.DeepEqual(rt.events, []string{"inspect"}) {
				t.Fatal("capture changed source state", rt.events)
			}
		}
	}
}

func TestSnapshotCancelledBeforeCaptureDoesNotStopSource(t *testing.T) {
	_, store, backend := captureFixture(t)
	rt := &quiesceRuntime{captureRuntime: backend, running: true}
	svc := New(rt, store)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	saved, err := svc.CaptureSnapshot(ctx, "resume")
	if !errors.Is(err, context.Canceled) || saved.ID != "" || !rt.running || len(store.trace.events) != 0 {
		t.Fatal(saved, err, rt.events, store.trace.events)
	}
}

func TestStoppedCopyCaptureRefusesRunningSourceWithoutMutation(t *testing.T) {
	for _, running := range []bool{false, true} {
		_, store, backend := captureFixture(t)
		rt := &quiesceRuntime{captureRuntime: backend, running: running}
		svc := New(rt, store)
		saved, err := svc.CaptureStoppedSnapshot(context.Background(), "resume")
		if running {
			if !errors.Is(err, core.ErrIncompatibleState) || saved.ID != "" || len(store.trace.events) != 0 {
				t.Fatal(saved, err, store.trace.events)
			}
		} else if err != nil || saved.State != "ready" {
			t.Fatal(saved, err)
		}
		if rt.running != running || !reflect.DeepEqual(rt.events, []string{"inspect"}) {
			t.Fatal(rt.events)
		}
	}
}
