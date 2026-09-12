package environment

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type recordingSnapshotProvider struct {
	fakeProvider
	source    core.SnapshotSource
	component core.SnapshotComponent
	calls     []string
}

func (p *recordingSnapshotProvider) PlanSnapshot(_ context.Context, s core.SnapshotSource, _ string) ([]core.SnapshotComponent, error) {
	p.source = s
	p.calls = append(p.calls, "plan")
	return []core.SnapshotComponent{{Role: "rootfs", NativeRef: "instance/saved", Owner: "owner", Binding: "opaque-plan", State: "planned"}}, nil
}
func (p *recordingSnapshotProvider) CreateSnapshotComponent(_ context.Context, s core.SnapshotSource, c core.SnapshotComponent) error {
	p.source = s
	p.component = c
	p.calls = append(p.calls, "create")
	return nil
}
func (p *recordingSnapshotProvider) VerifySnapshotComponent(_ context.Context, c core.SnapshotComponent) error {
	p.component = c
	p.calls = append(p.calls, "verify")
	return nil
}
func (p *recordingSnapshotProvider) DeleteSnapshotComponent(_ context.Context, c core.SnapshotComponent) error {
	p.component = c
	p.calls = append(p.calls, "delete")
	return nil
}
func TestSnapshotRoutingPreservesProviderAndOpaqueOwnership(t *testing.T) {
	chosen, other := &recordingSnapshotProvider{}, &recordingSnapshotProvider{}
	r, err := NewRouter("other", Register(testProvider, chosen), Register("other", other))
	if err != nil {
		t.Fatal(err)
	}
	router := NewBaseRouter(r)
	source := core.SnapshotSource{Environment: core.Environment{RuntimeRef: encodeRouteRef(testProvider, "source-native")}, InstanceID: "env-11111111111111111111111111111111"}
	cs, err := router.PlanSnapshot(context.Background(), source, "snapshot")
	if err != nil {
		t.Fatal(err)
	}
	c := cs[0]
	if c.NativeRef != encodeRouteRef(testProvider, "instance/saved") || chosen.source.Environment.RuntimeRef != "source-native" {
		t.Fatal(c, chosen.source)
	}
	if err := router.CreateSnapshotComponent(context.Background(), source, c); err != nil {
		t.Fatal(err)
	}
	if chosen.component.NativeRef != "instance/saved" || chosen.component.Binding != "opaque-plan" || chosen.source.InstanceID != source.InstanceID {
		t.Fatal("lost ownership")
	}
	c.State = "created"
	if err := router.VerifySnapshotComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if err := router.DeleteSnapshotComponent(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if len(chosen.calls) != 4 || len(other.calls) != 0 {
		t.Fatal(chosen.calls, other.calls)
	}
	source.Environment.RuntimeRef = encodeRouteRef("other", "source-native")
	if !errors.Is(router.CreateSnapshotComponent(context.Background(), source, c), core.ErrCapabilityStale) {
		t.Fatal("cross-provider create")
	}
	c.NativeRef = "instance/saved"
	if router.DeleteSnapshotComponent(context.Background(), c) == nil {
		t.Fatal("unrouted target adopted")
	}
	if len(chosen.calls) != 4 || len(other.calls) != 0 {
		t.Fatal("invalid request reached provider")
	}
}
