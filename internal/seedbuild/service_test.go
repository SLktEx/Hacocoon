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

const (
	fpA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fpB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	fpC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
)

type fakeBackend struct {
	parent       core.BaseRef
	toolingBuild int
	seedBuild    int
	plans        []BuildPlan
	seedErr      error
}

func (f *fakeBackend) ResolveParentBase(context.Context, core.BaseName) (core.BaseRef, error) {
	return f.parent, nil
}
func (f *fakeBackend) BuildToolingBase(context.Context, core.BaseRef) (BuildResult, error) {
	f.toolingBuild++
	return BuildResult{Revision: fpB, Alias: "tooling"}, nil
}
func (f *fakeBackend) BuildSeed(_ context.Context, plan BuildPlan) (BuildResult, error) {
	f.seedBuild++
	f.plans = append(f.plans, plan)
	if f.seedErr != nil {
		return BuildResult{}, f.seedErr
	}
	return BuildResult{Revision: fpC, Alias: "seed"}, nil
}

type fakeStats struct{ driver ociplugin.Driver }

func (f fakeStats) Driver() ociplugin.Driver {
	if f.driver == "" {
		return ociplugin.DriverNerdctl
	}
	return f.driver
}
func (fakeStats) SampleAll(context.Context, time.Duration) (ociplugin.SampleReport, error) {
	return ociplugin.SampleReport{Sampled: 2}, nil
}
func (fakeStats) Recommend(context.Context, time.Duration) ([]ociplugin.Recommendation, error) {
	return []ociplugin.Recommendation{
		{Reference: "docker.io/library/node:24", Digest: fpA, AutoPromote: true},
		{Reference: "docker.io/library/postgres:18", Digest: fpB, AutoPromote: false},
	}, nil
}

func TestBuildPublishesAndAdvancesCurrentOnlyAfterSuccess(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	backend := &fakeBackend{parent: core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}}
	service, err := New(backend, fakeStats{}, store)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return time.Unix(123, 0) }

	report, err := service.Build(context.Background(), "haco/ubuntu-26.04")
	if err != nil {
		t.Fatal(err)
	}
	if backend.toolingBuild != 1 || backend.seedBuild != 1 || report.ReusedToolingBase {
		t.Fatalf("backend=%#v report=%#v", backend, report)
	}
	if len(backend.plans) != 1 || len(backend.plans[0].Images) != 1 || backend.plans[0].Images[0].Reference != "docker.io/library/node:24" {
		t.Fatalf("plan=%#v", backend.plans)
	}
	current, ok, err := store.CurrentSeed(context.Background(), backend.parent)
	if err != nil || !ok || current != fpC {
		t.Fatalf("current=%q ok=%v err=%v", current, ok, err)
	}
}

func TestBuildRequiresNerdctlPluginDriver(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	backend := &fakeBackend{parent: core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}}
	service, err := New(backend, fakeStats{driver: ociplugin.DriverDocker}, store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Build(context.Background(), backend.parent.Name); !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("err=%v want ErrUnsupported", err)
	}
	if backend.toolingBuild != 0 || backend.seedBuild != 0 {
		t.Fatalf("Docker driver must fail before Seed side effects: %#v", backend)
	}
}

func TestBuildReusesToolingForSameParent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	parent := core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}
	if err := store.PutTooling(context.Background(), ToolingManifest{Parent: parent, ToolingRevision: fpB, BuiltAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	backend := &fakeBackend{parent: parent}
	service, _ := New(backend, fakeStats{}, store)
	report, err := service.Build(context.Background(), parent.Name)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ReusedToolingBase || backend.toolingBuild != 0 {
		t.Fatalf("report=%#v backend=%#v", report, backend)
	}
}

func TestParentMovementInvalidatesToolingAndCurrentPointers(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	oldParent := core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}
	if err := store.PutTooling(context.Background(), ToolingManifest{Parent: oldParent, ToolingRevision: fpB, BuiltAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := store.PutCurrent(context.Background(), Manifest{Parent: oldParent, ToolingRevision: fpB, SeedRevision: fpC, BuiltAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	newParent := core.BaseRef{Name: oldParent.Name, Revision: fpB}
	if _, ok, err := store.ToolingManifest(context.Background(), newParent); err != nil || ok {
		t.Fatalf("tooling ok=%v err=%v", ok, err)
	}
	if _, ok, err := store.CurrentSeed(context.Background(), newParent); err != nil || ok {
		t.Fatalf("seed ok=%v err=%v", ok, err)
	}
}

func TestFailedSeedBuildDoesNotAdvanceCurrent(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "seeds.json"))
	parent := core.BaseRef{Name: "haco/ubuntu-26.04", Revision: fpA}
	backend := &fakeBackend{parent: parent, seedErr: errors.New("publish failed")}
	service, _ := New(backend, fakeStats{}, store)
	if _, err := service.Build(context.Background(), parent.Name); err == nil {
		t.Fatal("expected build failure")
	}
	if _, ok, err := store.CurrentSeed(context.Background(), parent); err != nil || ok {
		t.Fatalf("current ok=%v err=%v", ok, err)
	}
}
