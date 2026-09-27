package creation

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type failedCreationDependency struct {
	*fixture
	fail string
}

func (f *failedCreationDependency) DefaultImage(ctx context.Context) (core.BaseName, error) {
	if f.fail == "default" {
		return "", core.ErrRuntimeUnavailable
	}
	return f.fixture.DefaultImage(ctx)
}
func (f *failedCreationDependency) SetDefaultImage(ctx context.Context, n core.BaseName, initial bool) error {
	if f.fail == "save-default" {
		return core.ErrRuntimeUnavailable
	}
	return f.fixture.SetDefaultImage(ctx, n, initial)
}
func (f *failedCreationDependency) GetEnvironment(ctx context.Context, n string) (core.Environment, error) {
	if f.fail == "lookup" {
		return core.Environment{}, core.ErrRuntimeUnavailable
	}
	return f.fixture.GetEnvironment(ctx, n)
}
func (f *failedCreationDependency) GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error) {
	if f.fail == "lease" {
		return core.WorkspaceLease{}, core.ErrRuntimeUnavailable
	}
	if f.fail == "lease-present" {
		return core.WorkspaceLease{}, nil
	}
	return core.WorkspaceLease{}, core.ErrNotFound
}
func (f *failedCreationDependency) LastOpened(ctx context.Context) (string, error) {
	if f.fail == "last" {
		return "", core.ErrRuntimeUnavailable
	}
	return f.fixture.LastOpened(ctx)
}
func (f *failedCreationDependency) RememberOpened(ctx context.Context, e core.Environment) error {
	if f.fail == "remember" {
		return core.ErrRuntimeUnavailable
	}
	return f.fixture.RememberOpened(ctx, e)
}
func (f *failedCreationDependency) List(ctx context.Context) ([]core.Environment, error) {
	if f.fail == "list" {
		return nil, core.ErrRuntimeUnavailable
	}
	return f.fixture.List(ctx)
}
func (f *failedCreationDependency) PrepareEnvironmentWorkspace(ctx context.Context, n string) (core.Workspace, error) {
	if f.fail == "work" {
		return core.Workspace{}, core.ErrRuntimeUnavailable
	}
	return f.fixture.PrepareEnvironmentWorkspace(ctx, n)
}

func TestCreationFailsClosedBeforePublishingOrStarting(t *testing.T) {
	for _, failure := range []string{"default", "lookup", "lease", "lease-present", "work"} {
		s, base := setupFixture()
		f := &failedCreationDependency{base, failure}
		s.Catalog, s.Workspaces, s.Environments = f, f, f
		_, err := s.Create(context.Background(), Request{Name: "new"})
		want := core.ErrRuntimeUnavailable
		if failure == "lease-present" {
			want = core.ErrRecoveryRequired
		}
		if !errors.Is(err, want) || len(f.envs) != 0 || strings.Contains(strings.Join(f.trace, ","), "create") {
			t.Fatal(failure, err, f.trace)
		}
	}
	for _, request := range []Request{{Name: "../new"}, {Name: "new", Image: "missing"}, {Name: "existing"}} {
		s, f := setupFixture()
		f.envs["existing"] = core.Environment{Name: "existing"}
		if _, err := s.Create(context.Background(), request); err == nil || strings.Contains(strings.Join(f.trace, ","), "repositories") {
			t.Fatal(request, err, f.trace)
		}
	}
	s, f := setupFixture()
	f.defaultImage = ""
	if _, err := s.Create(context.Background(), Request{Name: "new"}); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestOpenFailuresNeverRepairExistingOrStartUnrememberedEnvironment(t *testing.T) {
	for _, failure := range []string{"last", "list", "lookup", "remember"} {
		s, base := setupFixture()
		base.envs["work"] = core.Environment{Name: "work"}
		base.last = "work"
		f := &failedCreationDependency{base, failure}
		s.Catalog, s.Environments = f, f
		if _, err := s.OpenTarget(context.Background(), "", nil); !errors.Is(err, core.ErrRuntimeUnavailable) || len(f.trace) != 0 {
			t.Fatal(failure, err, f.trace)
		}
	}
	s, f := setupFixture()
	if _, err := s.OpenTarget(context.Background(), "existing", &Request{}); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
	f.createErr = core.ErrRuntimeUnavailable
	if _, err := s.OpenTarget(context.Background(), "", &Request{Name: "new"}); !errors.Is(err, core.ErrRuntimeUnavailable) || f.last != "" {
		t.Fatal(err, f.last)
	}
	f.createErr = nil
	f.trace = nil
	f.envs["older"] = core.Environment{Name: "older", CreatedAt: time.Unix(1, 0)}
	f.envs["newer"] = core.Environment{Name: "newer", CreatedAt: time.Unix(2, 0)}
	got, err := s.OpenTarget(context.Background(), "", nil)
	if err != nil || got.Name != "newer" || f.last != "newer" {
		t.Fatal(got, err, f.last)
	}
	if _, err := s.OpenTarget(context.Background(), "missing", nil); !errors.Is(err, core.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestDefaultReadAndPersistenceFailureDoNotChangePreference(t *testing.T) {
	s, base := setupFixture()
	f := &failedCreationDependency{base, "save-default"}
	s.Catalog = f
	if got, err := s.DefaultImage(context.Background(), ""); err != nil || got != "standard" {
		t.Fatal(got, err)
	}
	if _, err := s.DefaultImage(context.Background(), "tools"); !errors.Is(err, core.ErrRuntimeUnavailable) || base.defaultImage != "standard" {
		t.Fatal(err, base.defaultImage)
	}
}
