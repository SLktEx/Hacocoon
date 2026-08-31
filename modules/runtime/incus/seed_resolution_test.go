package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type fakeSeedResolver struct {
	parent   core.BaseRef
	revision core.BaseRevision
	present  bool
	err      error
}

func (f fakeSeedResolver) CurrentSeed(_ context.Context, parent core.BaseRef) (core.BaseRevision, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	if !f.present || parent != f.parent {
		return "", false, nil
	}
	return f.revision, true, nil
}

func TestBaseProviderUsesCurrentSeedForExactParentRevision(t *testing.T) {
	parent := core.BaseRef{Name: defaultBaseName, Revision: core.BaseRevision("sha256:" + testFingerprintA)}
	seedRevision := core.BaseRevision("sha256:" + testFingerprintB)
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case reflect.DeepEqual(args, []string{"image", "info", "images:ubuntu/26.04", "--format", "json"}):
			return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
		case reflect.DeepEqual(args, []string{"image", "info", testFingerprintB, "--project", defaultProject, "--format", "json"}):
			return host.Result{Stdout: `{"fingerprint":"` + testFingerprintB + `"}`}, nil
		default:
			return host.Result{}, errors.New("unexpected call")
		}
	}}
	provider, err := NewBaseProvider(New(runner), WithSeedResolver(fakeSeedResolver{parent: parent, revision: seedRevision, present: true}))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := provider.resolveBase(context.Background(), defaultBaseName)
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.usesSeed {
		t.Fatal("exact current Seed must mark the effective Base as Seed-backed")
	}
	if resolved.ref.Revision != seedRevision {
		t.Fatalf("revision=%q want=%q", resolved.ref.Revision, seedRevision)
	}
	if resolved.pinnedSource != testFingerprintB {
		t.Fatalf("pinnedSource=%q want current-server fingerprint %q", resolved.pinnedSource, testFingerprintB)
	}
}

func TestBaseProviderIgnoresSeedForMovedParentRevision(t *testing.T) {
	oldParent := core.BaseRef{Name: defaultBaseName, Revision: core.BaseRevision("sha256:" + testFingerprintA)}
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if reflect.DeepEqual(args, []string{"image", "info", "images:ubuntu/26.04", "--format", "json"}) {
			return host.Result{Stdout: `{"fingerprint":"` + testFingerprintB + `"}`}, nil
		}
		return host.Result{}, errors.New("unexpected call")
	}}
	provider, err := NewBaseProvider(New(runner), WithSeedResolver(fakeSeedResolver{
		parent: oldParent, revision: core.BaseRevision("sha256:" + testFingerprintB), present: true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := provider.resolveBase(context.Background(), defaultBaseName)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.usesSeed {
		t.Fatal("moved parent must not inherit nested OCI authority from a stale Seed")
	}
	want := core.BaseRevision("sha256:" + testFingerprintB)
	if resolved.ref.Revision != want {
		t.Fatalf("revision=%q want parent=%q", resolved.ref.Revision, want)
	}
}

func TestBaseProviderFailsClosedWhenCurrentSeedImageIsMissing(t *testing.T) {
	parent := core.BaseRef{Name: defaultBaseName, Revision: core.BaseRevision("sha256:" + testFingerprintA)}
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if reflect.DeepEqual(args, []string{"image", "info", "images:ubuntu/26.04", "--format", "json"}) {
			return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
		}
		if len(args) >= 3 && args[0] == "image" && args[1] == "info" && args[2] == testFingerprintB {
			return host.Result{}, errors.New("not found")
		}
		return host.Result{}, errors.New("unexpected call")
	}}
	provider, err := NewBaseProvider(New(runner), WithSeedResolver(fakeSeedResolver{
		parent: parent, revision: core.BaseRevision("sha256:" + testFingerprintB), present: true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.InspectBase(context.Background(), defaultBaseName); err == nil {
		t.Fatal("expected missing current Seed to fail closed")
	}
}
