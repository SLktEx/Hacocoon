package creation

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"testing"
	"time"
)

type fixture struct {
	defaultImage                    core.BaseName
	last                            string
	envs                            map[string]core.Environment
	trace                           []string
	spec                            core.EnvironmentSpec
	createErr, cleanupErr, startErr error
}

func (f *fixture) DefaultImage(context.Context) (core.BaseName, error) {
	f.trace = append(f.trace, "default")
	return f.defaultImage, nil
}
func (f *fixture) SetDefaultImage(_ context.Context, image core.BaseName, initial bool) error {
	if !initial || f.defaultImage == "" {
		f.defaultImage = image
	}
	return nil
}
func (f *fixture) LastOpened(context.Context) (string, error) { return f.last, nil }
func (f *fixture) RememberOpened(_ context.Context, e core.Environment) error {
	f.last = e.Name
	f.trace = append(f.trace, "remember")
	return nil
}
func (f *fixture) GetEnvironment(_ context.Context, n string) (core.Environment, error) {
	if e, ok := f.envs[n]; ok {
		return e, nil
	}
	return core.Environment{}, core.ErrNotFound
}
func (f *fixture) GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error) {
	return core.WorkspaceLease{}, core.ErrNotFound
}
func (f *fixture) InspectBase(_ context.Context, n core.BaseName) (core.BaseInfo, error) {
	f.trace = append(f.trace, "image:"+string(n))
	if n == "missing" {
		return core.BaseInfo{}, core.ErrNotFound
	}
	return core.BaseInfo{Name: n, Revision: "sha256:fixed"}, nil
}
func (f *fixture) PrepareEnvironmentWorkspace(_ context.Context, n string) (core.Workspace, error) {
	f.trace = append(f.trace, "repositories")
	return core.Workspace{ID: "owned", Path: "managed:" + n}, nil
}
func (f *fixture) Workspace(_ context.Context, n string) (core.Workspace, error) {
	f.trace = append(f.trace, n)
	return core.Workspace{ID: "volume", Path: "managed:" + n}, nil
}
func (f *fixture) Create(_ context.Context, s core.EnvironmentSpec) (core.Environment, error) {
	f.spec = s
	f.trace = append(f.trace, "create")
	if f.createErr != nil {
		return core.Environment{}, f.createErr
	}
	e := core.Environment{Name: s.Name, Workspace: core.Workspace{ID: s.ExpectedWorkspace, Path: s.WorkspacePath}, Base: &core.BaseRef{Name: s.Base}, OwnedWorkspace: s.OwnedWorkspace, Volume: s.Volume, CreatedAt: time.Now()}
	f.envs[e.Name] = e
	return e, nil
}
func (f *fixture) List(context.Context) ([]core.Environment, error) {
	var all []core.Environment
	for _, e := range f.envs {
		all = append(all, e)
	}
	return all, nil
}
func (f *fixture) StartForWorkspace(_ context.Context, n string, _ core.WorkspaceID) error {
	f.trace = append(f.trace, "start:"+n)
	return f.startErr
}
func (f *fixture) DeleteManagedWorkspace(context.Context, core.Workspace) error {
	f.trace = append(f.trace, "cleanup")
	return f.cleanupErr
}
func (f *fixture) CreateEnvironment(_ context.Context, id, n string) (core.Environment, error) {
	f.trace = append(f.trace, "snapshot:"+id)
	e := core.Environment{Name: n, Base: &core.BaseRef{Name: "saved-image"}, Workspace: core.Workspace{ID: "saved-copy"}}
	f.envs[n] = e
	return e, nil
}
func setupFixture() (*Service, *fixture) {
	f := &fixture{defaultImage: "standard", envs: map[string]core.Environment{}}
	return &Service{Catalog: f, Images: f, Workspaces: f, Environments: f, Snapshots: f}, f
}

func TestCreationSourcePrecedence(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request Request
		want    core.BaseName
		trace   []string
	}{
		{"default", Request{Name: "new"}, "standard", []string{"default", "image:standard", "repositories", "create"}},
		{"explicit", Request{Name: "new", Image: "tools"}, "tools", []string{"image:tools", "repositories", "create"}},
		{"snapshot", Request{Name: "new", Snapshot: "saved"}, "saved-image", []string{"snapshot:saved"}},
		{"volume", Request{Name: "new", Volume: "data"}, "standard", []string{"default", "image:standard", "volume-data", "create"}},
		{"image-volume", Request{Name: "new", Image: "tools", Volume: "data"}, "tools", []string{"image:tools", "volume-data", "create"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, f := setupFixture()
			e, err := s.Create(context.Background(), tc.request)
			if err != nil || e.Base.Name != tc.want || !reflect.DeepEqual(f.trace, tc.trace) {
				t.Fatalf("env=%+v trace=%v err=%v", e, f.trace, err)
			}
			if f.defaultImage != "standard" {
				t.Fatal("default changed")
			}
			if tc.request.Snapshot == "" && !f.spec.DeferStart {
				t.Fatal("create booted guest")
			}
		})
	}
}
func TestOpenCreatesStartsAndRemembersEmptyFirstEnvironment(t *testing.T) {
	s, f := setupFixture()
	e, err := s.OpenTarget(context.Background(), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if f.last != e.Name || !reflect.DeepEqual(f.trace, []string{"default", "image:standard", "repositories", "create", "remember", "start:" + e.Name}) {
		t.Fatalf("%v last=%s", f.trace, f.last)
	}
}
func TestOpenContinuesLastOpenedWithoutSourceResolution(t *testing.T) {
	s, f := setupFixture()
	f.envs["work"] = core.Environment{Name: "work", Workspace: core.Workspace{ID: "work"}}
	f.envs["other"] = core.Environment{Name: "other"}
	f.last = "work"
	e, err := s.OpenTarget(context.Background(), "", nil)
	if err != nil || e.Name != "work" || !reflect.DeepEqual(f.trace, []string{"remember", "start:work"}) {
		t.Fatalf("%+v %v %v", e, err, f.trace)
	}
}
func TestFailedStartRetainsCompletedEnvironmentForRetry(t *testing.T) {
	s, f := setupFixture()
	f.startErr = errors.New("start")
	e, err := s.OpenTarget(context.Background(), "", &Request{Name: "new"})
	if err == nil || f.last != "new" || f.envs[e.Name].Name != "new" {
		t.Fatalf("%+v %v", e, err)
	}
	f.startErr = nil
	f.trace = nil
	if _, err = s.OpenTarget(context.Background(), "", nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.trace, []string{"remember", "start:new"}) {
		t.Fatal(f.trace)
	}
}
func TestFailedCreationCleansOwnedWorkOnly(t *testing.T) {
	for _, volume := range []string{"", "data"} {
		s, f := setupFixture()
		f.createErr = errors.New("create")
		f.cleanupErr = core.ErrStorageBusy
		_, err := s.Create(context.Background(), Request{Name: "new", Volume: volume})
		if err == nil {
			t.Fatal("success")
		}
		if errors.Is(err, core.ErrStorageBusy) != (volume == "") {
			t.Fatalf("volume %q: %v", volume, err)
		}
	}
}
func TestDefaultChangeValidatesImageAndPreservesExistingEnvironments(t *testing.T) {
	s, f := setupFixture()
	f.envs["old"] = core.Environment{Name: "old", Base: &core.BaseRef{Name: "standard"}}
	if _, err := s.DefaultImage(context.Background(), "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
	if f.defaultImage != "standard" {
		t.Fatal("invalid default saved")
	}
	if _, err := s.DefaultImage(context.Background(), "tools"); err != nil {
		t.Fatal(err)
	}
	if f.envs["old"].Base.Name != "standard" {
		t.Fatal("existing environment mutated")
	}
}
func TestConflictingSnapshotInputsFailBeforeMutation(t *testing.T) {
	for _, r := range []Request{{Name: "new", Snapshot: "saved", Image: "tools"}, {Name: "new", Snapshot: "saved", Volume: "data"}} {
		s, f := setupFixture()
		if _, err := s.Create(context.Background(), r); !errors.Is(err, core.ErrInvalidArgument) || len(f.trace) != 0 {
			t.Fatalf("%v %v", err, f.trace)
		}
	}
}

func (f *fixture) DeleteOwnedWorkspace(ctx context.Context, w core.Workspace) error {
	return f.DeleteManagedWorkspace(ctx, w)
}
