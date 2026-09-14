package cache

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type catalogSpy struct {
	Generations
	calls int
	alter func(core.ResourceGeneration) core.ResourceGeneration
}

func (c *catalogSpy) EnsureResourceGeneration(ctx context.Context, name, kind, digest string) (core.ResourceGeneration, error) {
	c.calls++
	g, err := c.Generations.EnsureResourceGeneration(ctx, name, kind, digest)
	if c.alter != nil {
		g = c.alter(g)
	}
	return g, err
}

type repositories struct{ object gitrepo.Object }

func (r repositories) Get(kind, id string) (gitrepo.Object, error) {
	if kind != "work" || id != r.object.ID {
		return gitrepo.Object{}, core.ErrNotFound
	}
	return r.object, nil
}

func request() core.EnvironmentResourceRequest {
	return core.EnvironmentResourceRequest{EnvironmentID: "dev", InstanceID: "env-" + strings.Repeat("a", 32), Workspace: core.Workspace{ID: "work-one", Path: "/external"}, Base: "ubuntu"}
}

func area() Area {
	return Area{Name: "compiler", Path: "/root/.cache/compiler", Compatibility: "linux-amd64-compiler-1"}
}

func catalog(t *testing.T) *catalogSpy {
	t.Helper()
	return &catalogSpy{Generations: state.NewEnvironmentJSONStore(filepath.Join(t.TempDir(), "catalog.json"))}
}

func selectOne(t *testing.T, a Area, c Generations, req core.EnvironmentResourceRequest) core.EnvironmentResourceSelection {
	t.Helper()
	s, err := NewSelector(Configuration{Areas: []Area{a}}, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := s.Select(context.Background(), req)
	if err != nil || len(selected) != 1 {
		t.Fatal(selected, err)
	}
	return selected[0]
}

func TestScopeAndCompatibilitySelectIndependentGenerations(t *testing.T) {
	c, req, a := catalog(t), request(), area()
	first := selectOne(t, a, c, req)
	req.Base, req.EnvironmentID, req.InstanceID = "debian", "second", "env-"+strings.Repeat("b", 32)
	if got := selectOne(t, a, c, req); got.Origin != first.Origin {
		t.Fatal("compatible Base or Environment names split a generation")
	}
	req.Workspace.ID = "work-two"
	if got := selectOne(t, a, c, req); got.Origin == first.Origin {
		t.Fatal("default scope shared another Workspace")
	}
	a.Scope, a.Group = "shared", "build-team"
	shared := selectOne(t, a, c, req)
	req.Workspace.ID = "work-three"
	if got := selectOne(t, a, c, req); got.Origin != shared.Origin {
		t.Fatal("explicit compatible shared group did not reuse generation")
	}
	for _, change := range []func(*Area){
		func(a *Area) { a.Group = "other-team" },
		func(a *Area) { a.Compatibility = "linux-arm64-compiler-1" },
		func(a *Area) { a.Path += "-other" },
		func(a *Area) { a.Name = "different-tool" },
	} {
		changed := a
		change(&changed)
		if got := selectOne(t, changed, c, req); got.Origin == shared.Origin {
			t.Fatal("changed contract reused a generation", changed)
		}
	}
}

func TestInvalidConfigurationNeverTouchesCatalog(t *testing.T) {
	for _, change := range []func(*Area){
		func(a *Area) { a.Path = "/" },
		func(a *Area) { a.Path = "/root/../etc" },
		func(a *Area) { a.Path = "/root/a\\b" },
		func(a *Area) { a.Path = "/workspace/api/cache" },
		func(a *Area) { a.Path = "relative" },
		func(a *Area) { a.Repository, a.Path = "api", "../escape" },
		func(a *Area) { a.Repository, a.Path = "api", "/root/cache" },
		func(a *Area) { a.Name = "-option" },
		func(a *Area) { a.Compatibility = "" },
		func(a *Area) { a.Compatibility = "format\nsecret" },
		func(a *Area) { a.Scope = "unknown" },
		func(a *Area) { a.Scope = "shared" },
		func(a *Area) { a.Group = "group-without-shared-scope" },
	} {
		c, a := catalog(t), area()
		change(&a)
		if _, err := NewSelector(Configuration{Areas: []Area{a}}, c, nil); !errors.Is(err, core.ErrInvalidArgument) || c.calls != 0 {
			t.Fatal("invalid configuration accepted", a, err)
		}
	}
}

func TestWholeSelectionValidatedBeforeCatalogMutation(t *testing.T) {
	c, first, second := catalog(t), area(), area()
	second.Name, second.Path = "nested", first.Path+"/nested"
	s, err := NewSelector(Configuration{Areas: []Area{second, first}}, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Select(context.Background(), request()); !errors.Is(err, core.ErrInvalidArgument) || c.calls != 0 {
		t.Fatal("overlap allocated a generation", err, c.calls)
	}
	if _, err := NewSelector(Configuration{Areas: []Area{first, first}}, c, nil); err == nil {
		t.Fatal("duplicate key accepted")
	}
	if _, err := NewSelector(Configuration{Areas: make([]Area, core.MaxEnvironmentAttachments+1)}, c, nil); err == nil {
		t.Fatal("unbounded selection accepted")
	}
}

func TestManagedRepositoryPlacementUsesExactWorkspaceAndMember(t *testing.T) {
	owner := strings.Repeat("c", 32)
	object := gitrepo.Object{ID: "development", Owner: owner, Kind: "work", State: "ready", Members: []gitrepo.Object{
		{Repository: "api", Kind: "work", State: "ready"}, {Repository: "ui", Kind: "work", State: "ready"},
	}}
	a := area()
	a.Repository, a.Path = "api", "node_modules"
	req := request()
	req.Workspace = core.Workspace{ID: core.WorkspaceID("workspace:managed:" + owner), Path: "managed:development"}
	c := catalog(t)
	s, err := NewSelector(Configuration{Areas: []Area{a}}, c, repositories{object})
	if err != nil {
		t.Fatal(err)
	}
	selected, err := s.Select(context.Background(), req)
	if err != nil || len(selected) != 1 || selected[0].Target != "/workspace/api/node_modules" {
		t.Fatal(selected, err)
	}
	before := c.calls
	req.Workspace.ID += "wrong"
	if _, err := s.Select(context.Background(), req); !errors.Is(err, core.ErrCapabilityStale) || c.calls != before {
		t.Fatal("recycled Workspace accepted", err)
	}
	req = request()
	if got, err := s.Select(context.Background(), req); err != nil || len(got) != 0 || c.calls != before {
		t.Fatal("external work was adopted", got, err)
	}
}

func TestReadOnlyCancellationImmutableSettingsAndCatalogRefusal(t *testing.T) {
	c, config := catalog(t), Configuration{Areas: []Area{area()}}
	s, err := NewSelector(config, c, nil)
	if err != nil {
		t.Fatal(err)
	}
	config.Areas[0].Path = "/changed"
	req := request()
	req.ReadOnly = true
	if got, err := s.Select(context.Background(), req); err != nil || len(got) != 0 || c.calls != 0 {
		t.Fatal("read-only enrollment", got, err)
	}
	req.ReadOnly = false
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Select(ctx, req); !errors.Is(err, context.Canceled) || c.calls != 0 {
		t.Fatal("canceled selection mutated catalog", err)
	}
	got, err := s.Select(context.Background(), req)
	if err != nil || got[0].Target != area().Path {
		t.Fatal("caller changed frozen configuration", got, err)
	}
	c.alter = func(g core.ResourceGeneration) core.ResourceGeneration {
		g.Compatibility = strings.Repeat("f", 64)
		return g
	}
	if _, err := s.Select(context.Background(), req); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("mismatched full digest accepted", err)
	}
}
