package creation

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
)

type volumeFixture struct {
	*fixture
	objects   []gitrepo.Object
	fail      string
	collision bool
	saved     core.Snapshot
}

func (f *volumeFixture) LockResourceName(_ context.Context, name string) (func(), error) {
	f.trace = append(f.trace, "lock:"+name)
	if f.fail == "lock" {
		return nil, core.ErrStorageBusy
	}
	return func() { f.trace = append(f.trace, "unlock") }, nil
}
func (f *volumeFixture) GetSnapshot(context.Context, string) (core.Snapshot, error) {
	if f.collision {
		return f.saved, nil
	}
	if f.fail == "catalog" {
		return core.Snapshot{}, core.ErrRecoveryRequired
	}
	return core.Snapshot{}, core.ErrNotFound
}
func (f *volumeFixture) CreateEmptyWorkspace(_ context.Context, name string) (gitrepo.Object, error) {
	f.trace = append(f.trace, "empty:"+name)
	if f.fail == "empty" {
		return gitrepo.Object{}, core.ErrAlreadyExists
	}
	return gitrepo.Object{ID: name, Owner: "owned"}, nil
}
func (f *volumeFixture) ListWorkspaces(context.Context) ([]gitrepo.Object, error) {
	if f.fail == "list" {
		return nil, core.ErrRuntimeUnavailable
	}
	return f.objects, nil
}
func (f *volumeFixture) RestoreWorkspace(_ context.Context, name string, saved core.Snapshot) (gitrepo.Object, error) {
	f.trace = append(f.trace, "copy:"+name)
	if !reflect.DeepEqual(saved, f.saved) {
		return gitrepo.Object{}, core.ErrCapabilityStale
	}
	if f.fail == "copy" {
		return gitrepo.Object{}, core.ErrStorageBusy
	}
	return gitrepo.Object{ID: name, Owner: "independent"}, nil
}
func (f *volumeFixture) CaptureSnapshot(_ context.Context, name string) (core.Snapshot, error) {
	f.trace = append(f.trace, "capture:"+name)
	if f.fail == "capture" {
		return f.saved, core.ErrRuntimeUnavailable
	}
	return f.saved, nil
}
func (f *volumeFixture) DeleteSnapshot(ctx context.Context, id string) error {
	f.trace = append(f.trace, "delete-snapshot:"+id)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if _, ok := ctx.Deadline(); !ok {
		return core.ErrInvalidArgument
	}
	if f.fail == "cleanup" {
		return core.ErrRecoveryRequired
	}
	return nil
}

func TestVolumeCreationCopiesDataAndCleansTemporarySnapshot(t *testing.T) {
	for _, tc := range []struct {
		fail, source string
		want         error
		trace        []string
	}{
		{"", "", nil, []string{"lock:data", "empty:volume-data", "unlock"}},
		{"", "work", nil, []string{"lock:data", "capture:work", "copy:volume-data", "delete-snapshot:saved", "unlock"}},
		{"empty", "", core.ErrAlreadyExists, []string{"lock:data", "empty:volume-data", "unlock"}},
		{"capture", "work", core.ErrRuntimeUnavailable, []string{"lock:data", "capture:work", "delete-snapshot:saved", "unlock"}},
		{"copy", "work", core.ErrStorageBusy, []string{"lock:data", "capture:work", "copy:volume-data", "delete-snapshot:saved", "unlock"}},
		{"cleanup", "work", core.ErrRecoveryRequired, []string{"lock:data", "capture:work", "copy:volume-data", "delete-snapshot:saved", "unlock"}},
		{"lock", "", core.ErrStorageBusy, []string{"lock:data"}},
		{"catalog", "", core.ErrRecoveryRequired, []string{"lock:data", "unlock"}},
	} {
		t.Run(tc.fail+"/"+tc.source, func(t *testing.T) {
			s, base := setupFixture()
			f := &volumeFixture{fixture: base, fail: tc.fail, saved: core.Snapshot{ID: "saved"}}
			s.Workspaces, s.Environments, s.Catalog = f, f, f
			got, err := s.CreateVolume(context.Background(), "data", tc.source)
			if !errors.Is(err, tc.want) || !reflect.DeepEqual(f.trace, tc.trace) {
				t.Fatalf("%+v %v %v", got, err, f.trace)
			}
			if err == nil && (got.Name != "data" || got.Workspace.Path != "managed:volume-data") {
				t.Fatal(got)
			}
			if tc.source != "" && err == nil && got.Workspace.ID != "workspace:managed:independent" {
				t.Fatal("copy retained source identity", got)
			}
		})
	}
}

func TestVolumeNamespaceAndLifecycleRefusals(t *testing.T) {
	s, base := setupFixture()
	for _, name := range []string{"../data", strings.Repeat("a", 42)} {
		if _, err := s.CreateVolume(context.Background(), name, ""); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
		if _, err := s.InspectVolume(context.Background(), name); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
	if _, err := s.CreateVolume(context.Background(), "data", ""); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := s.Volumes(context.Background()); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	f := &volumeFixture{fixture: base, collision: true}
	s.Workspaces, s.Environments, s.Catalog = f, f, f
	if _, err := s.CreateVolume(context.Background(), "data", ""); !errors.Is(err, core.ErrAlreadyExists) || !reflect.DeepEqual(f.trace, []string{"lock:data", "unlock"}) {
		t.Fatal(err, f.trace)
	}
	f.collision = false
	f.objects = []gitrepo.Object{{ID: "env-private", Owner: "private"}, {ID: "volume-data", Owner: "volume-owner"}}
	volumes, err := s.Volumes(context.Background())
	if err != nil || len(volumes) != 1 || volumes[0].Name != "data" || volumes[0].Workspace.ID != "workspace:managed:volume-owner" {
		t.Fatal(volumes, err)
	}
	f.fail = "list"
	if _, err := s.Volumes(context.Background()); !errors.Is(err, core.ErrRuntimeUnavailable) {
		t.Fatal(err)
	}
	base.cleanupErr = core.ErrStorageBusy
	if err := s.DeleteVolume(context.Background(), "data"); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal("leased Volume deletion accepted", err)
	}
	base.cleanupErr = nil
	if err := s.DeleteVolume(context.Background(), "data"); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteVolume(context.Background(), "../data"); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	s.Environments = base
	if _, err := s.CreateVolume(context.Background(), "data", "work"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}
