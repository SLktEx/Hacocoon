package environmentcopy

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/snapshotrestore"
	"reflect"
	"strings"
	"testing"
)

type fixture struct {
	fail   string
	steps  []string
	target string
	cancel context.CancelFunc
}

func (f *fixture) GetEnvironment(context.Context, string) (core.Environment, error) {
	if f.fail == "exists" {
		return core.Environment{}, nil
	}
	return core.Environment{}, core.ErrNotFound
}
func (f *fixture) GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error) {
	if f.fail == "lease" {
		return core.WorkspaceLease{}, nil
	}
	return core.WorkspaceLease{}, core.ErrNotFound
}
func (f *fixture) CaptureStoppedSnapshot(context.Context, string) (core.Snapshot, error) {
	f.steps = append(f.steps, "capture")
	if f.fail == "running" {
		return core.Snapshot{}, core.ErrIncompatibleState
	}
	s := core.Snapshot{ID: "snap-" + strings.Repeat("a", 32), State: "ready"}
	if f.fail == "capture" {
		return s, core.ErrRecoveryRequired
	}
	return s, nil
}
func (f *fixture) RestoreSnapshot(_ context.Context, id, target string) (snapshotrestore.Result, error) {
	f.steps = append(f.steps, "restore")
	f.target = target
	r := snapshotrestore.Result{Environment: target, Workspace: "owned-copy", State: "running"}
	if f.cancel != nil {
		f.cancel()
		return r, context.Canceled
	}
	if f.fail == "early-restore" {
		return snapshotrestore.Result{}, core.ErrNotFound
	}
	if f.fail == "restore" {
		return r, core.ErrRuntimeUnavailable
	}
	return r, nil
}
func (f *fixture) DeleteSnapshot(ctx context.Context, id string) error {
	f.steps = append(f.steps, "cleanup")
	if ctx.Err() != nil || id != "snap-"+strings.Repeat("a", 32) {
		return errors.New("wrong cleanup context or identity")
	}
	if f.fail == "cleanup" {
		return core.ErrStorageBusy
	}
	return nil
}
func TestCopyUsesStoppedCaptureAndOwnedCleanup(t *testing.T) {
	for _, failure := range []string{"", "exists", "lease", "running", "capture", "restore", "early-restore", "cleanup", "cancel"} {
		t.Run(failure, func(t *testing.T) {
			f := &fixture{fail: failure}
			s := Service{Catalog: f, Snapshots: f, Restorer: f}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if failure == "cancel" {
				f.cancel = cancel
			}
			got, err := s.CopyEnvironment(ctx, "dev", "")
			if (err != nil) != (failure != "") {
				t.Fatal(got, err)
			}
			if failure == "exists" || failure == "lease" {
				if len(f.steps) != 0 {
					t.Fatal(f.steps)
				}
				return
			}
			if failure == "running" {
				if !reflect.DeepEqual(f.steps, []string{"capture"}) {
					t.Fatal(f.steps)
				}
				return
			}
			expected := []string{"capture", "restore", "cleanup"}
			if failure == "capture" {
				expected = []string{"capture", "cleanup"}
			}
			if !reflect.DeepEqual(f.steps, expected) {
				t.Fatal(f.steps)
			}
			if (got.TemporarySnapshot != "") != (failure == "cleanup") {
				t.Fatal(got)
			}
			if failure == "cleanup" && (!errors.Is(err, core.ErrRecoveryRequired) || got.State != "running") {
				t.Fatal(got, err)
			}
			if failure != "capture" && (f.target != "dev-copy" || got.Environment != "dev-copy") {
				t.Fatal(f.target)
			}
		})
	}
}
func TestCopyInvalidNamesNeverCapture(t *testing.T) {
	f := &fixture{}
	s := Service{Catalog: f, Snapshots: f, Restorer: f}
	for _, pair := range [][2]string{{"-option", "new"}, {"dev", "dev"}, {"dev", "../bad"}} {
		if _, err := s.CopyEnvironment(context.Background(), pair[0], pair[1]); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(pair, err)
		}
	}
	if len(f.steps) != 0 {
		t.Fatal(f.steps)
	}
	got, err := s.CopyEnvironment(context.Background(), strings.Repeat("a", 57), "")
	if err != nil || len(got.Environment) != 57 {
		t.Fatal(got, err)
	}
}
