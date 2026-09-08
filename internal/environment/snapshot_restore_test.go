package environment

import (
	"context"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type recordingRestoreProvider struct {
	recordingSnapshotProvider
	saved core.Snapshot
}

func (p *recordingRestoreProvider) PlanSnapshotRestore(_ context.Context, s core.Snapshot, _ string) ([]core.SnapshotComponent, error) {
	p.saved = s
	p.calls = append(p.calls, "restore-plan")
	return []core.SnapshotComponent{{NativeRef: "instance/restored", Binding: "opaque", State: "planned"}}, nil
}
func (p *recordingRestoreProvider) CreateRestoreComponent(_ context.Context, s core.Snapshot, c core.SnapshotComponent) error {
	p.saved = s
	p.component = c
	p.calls = append(p.calls, "restore-create")
	return nil
}
func (p *recordingRestoreProvider) VerifyRestoreComponent(_ context.Context, c core.SnapshotComponent) error {
	p.component = c
	p.calls = append(p.calls, "restore-verify")
	return nil
}
func (p *recordingRestoreProvider) DeleteRestoreComponent(_ context.Context, c core.SnapshotComponent) error {
	p.component = c
	p.calls = append(p.calls, "restore-delete")
	return nil
}
func TestRestoreRouterRequiresSingleExactProvider(t *testing.T) {
	p, other := &recordingRestoreProvider{}, &recordingRestoreProvider{}
	r, err := NewRouter("other", Register(testProvider, p), Register("other", other))
	if err != nil {
		t.Fatal(err)
	}
	saved := core.Snapshot{Source: core.SnapshotSource{Environment: core.Environment{RuntimeRef: encodeRouteRef(testProvider, "old-env")}}, Components: []core.SnapshotComponent{{NativeRef: encodeRouteRef(testProvider, "instance/saved"), Binding: "immutable"}}}
	cs, err := NewBaseRouter(r).PlanSnapshotRestore(context.Background(), saved, "restore")
	if err != nil {
		t.Fatal(err)
	}
	if p.saved.Components[0].NativeRef != "instance/saved" || saved.Components[0].NativeRef == "instance/saved" {
		t.Fatal("source decode mutated caller")
	}
	c := cs[0]
	if err := r.CreateRestoreComponent(context.Background(), saved, c); err != nil {
		t.Fatal(err)
	}
	if p.component.NativeRef != "instance/restored" || p.component.Binding != "opaque" {
		t.Fatal("lost target binding")
	}
	if err := r.VerifyRestoreComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if err := r.DeleteRestoreComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	saved.Components[0].NativeRef = encodeRouteRef("other", "instance/saved")
	if r.CreateRestoreComponent(context.Background(), saved, c) == nil {
		t.Fatal("cross provider source accepted")
	}
	c.NativeRef = "instance/restored"
	if r.DeleteRestoreComponent(context.Background(), c) == nil {
		t.Fatal("legacy target accepted")
	}
	if len(p.calls) != 4 || len(other.calls) != 0 {
		t.Fatal("invalid routing reached provider", p.calls, other.calls)
	}
}
