package environment

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type committingProvider struct{ publishingProvider }

func (p *committingProvider) CommitImage(ctx context.Context, e core.Environment, l core.WorkspaceLease, n core.BaseName) (core.BaseInfo, error) {
	return p.PublishBase(ctx, e, l, n)
}

type snapshotImageProvider struct {
	recordingSnapshotProvider
	failure error
}

func (p *snapshotImageProvider) SnapshotImage(_ context.Context, c core.SnapshotComponent) (core.BaseRef, error) {
	p.component = c
	p.calls = append(p.calls, "image")
	return core.BaseRef{Name: "saved", Revision: "immutable"}, p.failure
}

func TestCommitImageRoutesExactLeaseAndPreservesProviderFailure(t *testing.T) {
	p := &committingProvider{}
	other := &committingProvider{}
	r, err := NewRouter("other", Register("other", other), Register(testProvider, p))
	if err != nil {
		t.Fatal(err)
	}
	env := core.Environment{Name: "work", RuntimeRef: encodeRouteRef(testProvider, "native-work")}
	lease := core.WorkspaceLease{RuntimeRef: env.RuntimeRef, InstanceID: "owner"}
	for _, failure := range []error{nil, core.ErrRuntimeUnavailable} {
		p.failure = failure
		got, err := r.CommitImage(context.Background(), env, lease, "target")
		if !errors.Is(err, failure) || got.Name != "target" || p.env.RuntimeRef != "native-work" || p.lease.RuntimeRef != "native-work" || p.lease.InstanceID != lease.InstanceID || other.calls != 0 {
			t.Fatal(got, err, p, other)
		}
	}
	bad := lease
	bad.RuntimeRef = "replacement"
	calls := p.calls
	if _, err := r.CommitImage(context.Background(), env, bad, "target"); !errors.Is(err, core.ErrCapabilityStale) || p.calls != calls {
		t.Fatal(err)
	}
	limited, err := NewRouter(testProvider, Register(testProvider, &fakeProvider{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := limited.CommitImage(context.Background(), env, lease, "target"); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	env.RuntimeRef = encodeRouteRef("missing", "native")
	lease.RuntimeRef = env.RuntimeRef
	if _, err := r.CommitImage(context.Background(), env, lease, "target"); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func TestSnapshotImageUsesSavedProviderRouteAndOpaqueOwnership(t *testing.T) {
	p := &snapshotImageProvider{}
	r, err := NewRouter(testProvider, Register(testProvider, p))
	if err != nil {
		t.Fatal(err)
	}
	c := core.SnapshotComponent{Role: "rootfs", NativeRef: encodeRouteRef(testProvider, "saved-native"), Owner: "saved-owner", Binding: "saved-config"}
	for _, failure := range []error{nil, core.ErrNotFound} {
		p.failure = failure
		got, err := r.SnapshotImage(context.Background(), c)
		want := c
		want.NativeRef = "saved-native"
		if !errors.Is(err, failure) || got.Name != "saved" || !reflect.DeepEqual(p.component, want) {
			t.Fatal(got, err, p.component)
		}
	}
	if _, err := r.SnapshotImage(context.Background(), core.SnapshotComponent{NativeRef: "unwrapped"}); err == nil {
		t.Fatal("unowned route accepted")
	}
	limited, err := NewRouter(testProvider, Register(testProvider, &recordingSnapshotProvider{}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := limited.SnapshotImage(context.Background(), c); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
}
