package seedbuild

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

type mutableDeletionStats struct {
	deleted map[string]bool
}

func (mutableDeletionStats) Driver() ociplugin.Driver { return ociplugin.DriverNerdctl }
func (mutableDeletionStats) SampleAll(context.Context, time.Duration) (ociplugin.SampleReport, error) {
	return ociplugin.SampleReport{Sampled: 1}, nil
}
func (mutableDeletionStats) Recommend(context.Context, time.Duration) ([]ociplugin.Recommendation, error) {
	return []ociplugin.Recommendation{{Reference: "docker.io/library/node:24", Digest: fpA, AutoPromote: true}}, nil
}
func (f mutableDeletionStats) IsImageDeleted(_ context.Context, reference, digest string) (bool, error) {
	return f.deleted[reference+"@"+digest], nil
}

type maintenanceFakeBackend struct {
	fakeBackend
	calls  []bool
	report MaintenanceReport
	err    error
}

func (f *maintenanceFakeBackend) MaintainSeedArtifacts(_ context.Context, _ MaintenanceProtection, recoverBuilders bool) (MaintenanceReport, error) {
	f.calls = append(f.calls, recoverBuilders)
	return f.report, f.err
}

func TestPinnedImageIsMergedWithAutomaticSelection(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	backend := &fakeBackend{parent: core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}}
	stats := mutableDeletionStats{deleted: map[string]bool{}}
	service, err := New(backend, stats, store)
	if err != nil {
		t.Fatal(err)
	}
	pinIdentity := "docker.io/library/postgres:18@" + fpB
	if _, err := service.Pin(context.Background(), backend.parent.Name, pinIdentity); err != nil {
		t.Fatal(err)
	}
	report, err := service.Build(context.Background(), backend.parent.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Images) != 2 {
		t.Fatalf("images=%#v want automatic + explicit pin", report.Images)
	}
	if report.Images[0].String() != "docker.io/library/node:24@"+fpA || report.Images[1].String() != pinIdentity {
		t.Fatalf("images=%#v", report.Images)
	}
}

func TestDeletionTombstoneOverridesExistingPin(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	backend := &fakeBackend{parent: core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}}
	stats := mutableDeletionStats{deleted: map[string]bool{}}
	service, err := New(backend, stats, store)
	if err != nil {
		t.Fatal(err)
	}
	identity := "docker.io/library/postgres:18@" + fpB
	if _, err := service.Pin(context.Background(), backend.parent.Name, identity); err != nil {
		t.Fatal(err)
	}
	stats.deleted[identity] = true
	if _, err := service.Build(context.Background(), backend.parent.Name); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
	if backend.toolingBuild != 0 || backend.seedBuild != 0 {
		t.Fatalf("tombstone conflict must fail before backend mutation: %#v", backend)
	}
}

func TestPinRejectsTombstonedIdentityUntilReenabled(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	backend := &fakeBackend{parent: core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}}
	identity := "docker.io/library/postgres:18@" + fpB
	stats := mutableDeletionStats{deleted: map[string]bool{identity: true}}
	service, _ := New(backend, stats, store)
	if _, err := service.Pin(context.Background(), backend.parent.Name, identity); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
	pins, err := service.Pins(context.Background(), backend.parent.Name)
	if err != nil {
		t.Fatal(err)
	}
	if len(pins) != 0 {
		t.Fatalf("pins=%#v want none", pins)
	}
}

func TestUnpinIsIdempotent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	backend := &fakeBackend{parent: core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}}
	service, _ := New(backend, mutableDeletionStats{deleted: map[string]bool{}}, store)
	identity := "docker.io/library/postgres:18@" + fpB
	if _, err := service.Pin(context.Background(), backend.parent.Name, identity); err != nil {
		t.Fatal(err)
	}
	removed, err := service.Unpin(context.Background(), backend.parent.Name, identity)
	if err != nil || !removed {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	removed, err = service.Unpin(context.Background(), backend.parent.Name, identity)
	if err != nil || removed {
		t.Fatalf("second removed=%v err=%v", removed, err)
	}
}

func TestMaintenanceUsesCurrentManifestProtectionAndRecoveryMode(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	parent := core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}
	if err := store.PutTooling(context.Background(), ToolingManifest{Parent: parent, ToolingRevision: fpB, ToolingAlias: "hacocoon-tooling-haco-ubuntu-26-04-1", BuiltAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutCurrent(context.Background(), Manifest{Parent: parent, ToolingRevision: fpB, SeedRevision: fpC, SeedAlias: "hacocoon-seed-haco-ubuntu-26-04-2", BuiltAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	backend := &maintenanceFakeBackend{fakeBackend: fakeBackend{parent: parent}}
	service, _ := New(backend, fakeStats{}, store)
	if _, err := service.GC(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Recover(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(backend.calls) != 2 || backend.calls[0] || !backend.calls[1] {
		t.Fatalf("maintenance modes=%#v", backend.calls)
	}
	protection, err := store.MaintenanceProtection(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(protection.Revisions) != 2 || len(protection.Aliases) != 2 {
		t.Fatalf("protection=%#v", protection)
	}
}
