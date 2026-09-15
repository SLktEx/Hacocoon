package basemanage

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"reflect"
	"testing"
)

type fixture struct {
	id        Identity
	envs      []core.Environment
	snapshots []core.Snapshot
	deleted   bool
	listErr   error
	envsErr   error
	snapsErr  error
}

func (f *fixture) ListBaseImages(context.Context) ([]Image, error) {
	return []Image{{Identity: f.id}}, f.listErr
}
func (f *fixture) DeleteBaseImage(ctx context.Context, id Identity, check func(context.Context) error) error {
	if err := check(ctx); err != nil {
		return err
	}
	f.deleted = true
	return nil
}
func (f *fixture) ListEnvironments(context.Context) ([]core.Environment, error) {
	return f.envs, f.envsErr
}
func (f *fixture) ListSnapshots(context.Context) ([]core.Snapshot, error) {
	return f.snapshots, f.snapsErr
}
func TestCatalogUsersBlockButSnapshotProvenanceDoesNot(t *testing.T) {
	f := &fixture{id: Identity{Name: "tools", Fingerprint: "fingerprint"}}
	base := &core.BaseRef{Name: "old-name", Revision: "sha256:fingerprint"}
	f.envs = []core.Environment{{Name: "dev", Base: base}}
	f.snapshots = []core.Snapshot{{ID: "saved", Source: core.SnapshotSource{Environment: core.Environment{Base: base}}}}
	s := &Service{Backend: f, Catalog: f}
	ctx := context.Background()
	list, err := s.List(ctx)
	if err != nil || len(list) != 1 || len(list[0].Environments) != 1 || len(list[0].IndependentSnapshots) != 1 {
		t.Fatal(list, err)
	}
	if err = s.Delete(ctx, f.id); !errors.Is(err, core.ErrStorageBusy) || f.deleted {
		t.Fatal(err)
	}
	f.envs = nil
	if err = s.Delete(ctx, f.id); err != nil || !f.deleted || len(f.snapshots) != 1 {
		t.Fatal(err)
	}
}

func TestUnavailableCatalogFailsClosed(t *testing.T) {
	ctx := context.Background()
	for _, service := range []*Service{nil, {}, {Backend: &fixture{}}, {Catalog: &fixture{}}} {
		if _, err := service.List(ctx); !errors.Is(err, core.ErrUnsupported) {
			t.Fatalf("list without dependencies: %v", err)
		}
		if err := service.Delete(ctx, Identity{}); !errors.Is(err, core.ErrUnsupported) {
			t.Fatalf("delete without dependencies: %v", err)
		}
	}
	failure := errors.New("catalog unavailable")
	for _, source := range []string{"native images", "environments", "snapshots"} {
		t.Run(source, func(t *testing.T) {
			f := &fixture{}
			switch source {
			case "native images":
				f.listErr = failure
			case "environments":
				f.envsErr = failure
			case "snapshots":
				f.snapsErr = failure
			}
			s := &Service{Backend: f, Catalog: f}
			if images, err := s.List(ctx); !errors.Is(err, failure) || images != nil {
				t.Fatalf("reported complete inventory despite read failure: %v, %v", images, err)
			}
			if source == "environments" {
				if err := s.Delete(ctx, f.id); !errors.Is(err, failure) || f.deleted {
					t.Fatalf("deleted without current ownership inventory: %v", err)
				}
			}
		})
	}
}

func TestReferencesUseImmutableRevisionAndSortedNames(t *testing.T) {
	wanted := &core.BaseRef{Name: "renamed", Revision: "sha256:one"}
	f := &fixture{
		id:        Identity{Name: "tools", Fingerprint: "one"},
		envs:      []core.Environment{{Name: "z", Base: wanted}, {Name: "none"}, {Name: "a", Base: wanted}, {Name: "different", Base: &core.BaseRef{Name: "tools", Revision: "sha256:two"}}},
		snapshots: []core.Snapshot{{ID: "z", Source: core.SnapshotSource{Environment: core.Environment{Base: wanted}}}, {ID: "none"}, {ID: "a", Source: core.SnapshotSource{Environment: core.Environment{Base: wanted}}}},
	}
	images, err := (&Service{Backend: f, Catalog: f}).List(context.Background())
	if err != nil || len(images) != 1 {
		t.Fatal(images, err)
	}
	if got := images[0]; !reflect.DeepEqual(got.Environments, []string{"a", "z"}) || !reflect.DeepEqual(got.IndependentSnapshots, []string{"a", "z"}) {
		t.Fatalf("incorrect revision users: %+v", got)
	}
}
