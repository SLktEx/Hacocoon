package basemanage

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"testing"
)

type fixture struct {
	id        Identity
	envs      []core.Environment
	snapshots []core.Snapshot
	deleted   bool
}

func (f *fixture) ListBaseImages(context.Context) ([]Image, error) {
	return []Image{{Identity: f.id}}, nil
}
func (f *fixture) DeleteBaseImage(ctx context.Context, id Identity, check func(context.Context) error) error {
	if err := check(ctx); err != nil {
		return err
	}
	f.deleted = true
	return nil
}
func (f *fixture) ListEnvironments(context.Context) ([]core.Environment, error) { return f.envs, nil }
func (f *fixture) ListSnapshots(context.Context) ([]core.Snapshot, error)       { return f.snapshots, nil }
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
