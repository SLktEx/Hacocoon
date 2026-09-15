package cache

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type emptyFixture struct {
	*maintenanceFixture
	calls     []core.EnvironmentAttachment
	snapshots []core.Snapshot
	fail      string
}

func (f *emptyFixture) ListSnapshots(context.Context) ([]core.Snapshot, error) {
	return f.snapshots, nil
}
func (f *emptyFixture) EmptyEnvironmentResource(_ context.Context, name string, area core.EnvironmentAttachment) error {
	if name != f.env.Name {
		return core.ErrCapabilityStale
	}
	f.calls = append(f.calls, area)
	if area.Key == f.fail {
		return core.ErrRecoveryRequired
	}
	return nil
}
func emptyTestWorkflow() (*Workflow, *emptyFixture) {
	w, m := maintenanceTestWorkflow()
	f := &emptyFixture{maintenanceFixture: m}
	for _, key := range []string{"compiler", "packages"} {
		r := m.resources[0]
		r.ID = "env-data:" + key
		r.EnvironmentInstance = "parent"
		r.SourceOnly = false
		a := m.env.Attachments[0]
		a.Key = key
		a.Resource = r.Ref()
		if key == "compiler" {
			m.env.Attachments[0] = a
		} else {
			m.env.Attachments = append(m.env.Attachments, a)
		}
		m.resources = append(m.resources, r)
	}
	w.Catalog = f
	w.Emptier = f
	return w, f
}
func TestEmptyCacheReviewBindsScopeAndExactOwner(t *testing.T) {
	for _, mode := range []string{"owner", "name", "snapshot", "area"} {
		t.Run(mode, func(t *testing.T) {
			w, f := emptyTestWorkflow()
			scope := EmptyScope{All: true}
			p, err := w.PreviewEmpty(context.Background(), scope)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "owner":
				f.resources[1].Owner = "changed"
				f.env.Attachments[0].Resource = f.resources[1].Ref()
			case "name":
				f.env.Name = "recreated"
			case "area":
				f.env.Attachments = f.env.Attachments[:1]
			case "snapshot":
				s := core.Snapshot{}
				s.Source.Environment = f.env
				f.snapshots = []core.Snapshot{s}
			}
			if _, err := w.Empty(context.Background(), scope, p.Revision); !errors.Is(err, core.ErrCapabilityStale) || len(f.calls) != 0 {
				t.Fatal("stale review acted", err, f.calls)
			}
		})
	}
}
func TestEmptyCachePartialResultsKeepIndependentAreasAndSources(t *testing.T) {
	w, f := emptyTestWorkflow()
	scope := EmptyScope{Environment: "dev"}
	f.fail = "compiler"
	p, err := w.PreviewEmpty(context.Background(), scope)
	if err != nil || len(p.Areas) != 2 {
		t.Fatal(p, err)
	}
	result, err := w.Empty(context.Background(), scope, p.Revision)
	if !errors.Is(err, core.ErrRecoveryRequired) || len(f.calls) != 2 || result.Areas[0].State != "failed" || result.Areas[1].State != "empty" {
		t.Fatal(result, err, f.calls)
	}
	if f.calls[0] != f.env.Attachments[0] || f.calls[1] != f.env.Attachments[1] || f.resets != 0 || len(f.deleted) != 0 {
		t.Fatal("wrong scope")
	}
	p, err = w.PreviewEmpty(context.Background(), EmptyScope{Environment: "dev", Area: "packages"})
	if err != nil || len(p.Areas) != 1 || p.Areas[0].Name != "packages" {
		t.Fatal(p, err)
	}
}
