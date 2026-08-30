package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const compatImageFingerprint = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
const compatVMFingerprint = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

func TestResolveParentBaseSelectsContainerWhenAliasAlsoHasVM(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, args []string) (host.Result, error) {
		if call != 0 {
			t.Fatalf("unexpected call %d: %#v", call, args)
		}
		assertStringSlice(t, args, []string{"image", "list", "images:", "ubuntu/26.04", "--format", "csv", "-c", "L,F,T"})
		return host.Result{Stdout: "\"ubuntu/26.04\nubuntu/latest\"," + compatImageFingerprint + ",CONTAINER\n" +
			"ubuntu/26.04," + compatVMFingerprint + ",VIRTUAL-MACHINE\n"}, nil
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	resolved, err := provider.resolveParentBase(context.Background(), defaultBaseName)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.ref.Revision != core.BaseRevision("sha256:"+compatImageFingerprint) {
		t.Fatalf("revision = %q", resolved.ref.Revision)
	}
	if resolved.pinnedSource != "images:"+compatImageFingerprint {
		t.Fatalf("pinned source = %q", resolved.pinnedSource)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestImageFingerprintMatchesContainerFingerprintPrefix(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, args []string) (host.Result, error) {
		if call != 0 {
			t.Fatalf("unexpected call %d: %#v", call, args)
		}
		assertStringSlice(t, args, []string{"image", "list", "local:", compatImageFingerprint, "--format", "csv", "-c", "L,F,T", "--project", "hacocoon"})
		return host.Result{Stdout: "," + compatImageFingerprint + ",CONTAINER\n"}, nil
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	got, err := provider.imageFingerprint(context.Background(), "local:"+compatImageFingerprint, "hacocoon")
	if err != nil {
		t.Fatal(err)
	}
	if got != compatImageFingerprint {
		t.Fatalf("fingerprint = %q", got)
	}
}

func TestImageFingerprintIgnoresVMOnlyMatch(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{Stdout: "ubuntu/26.04," + compatVMFingerprint + ",VIRTUAL-MACHINE\n"}, nil
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.imageFingerprint(context.Background(), "images:ubuntu/26.04", "")
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}

func TestImageFingerprintPropagatesListFailure(t *testing.T) {
	permissionErr := errors.New("permission denied")
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{ExitCode: 1, Stderr: "Error: permission denied\n"}, permissionErr
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	_, err = provider.imageFingerprint(context.Background(), "images:ubuntu/26.04", "")
	if !errors.Is(err, permissionErr) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("unexpected calls: %#v", runner.calls)
	}
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("args = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("args = %#v, want %#v", got, want)
		}
	}
}
