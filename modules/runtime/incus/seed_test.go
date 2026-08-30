package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/seedbuild"
)

func TestVerifyBuilderHasNoNIC(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) == 2 && args[0] == "query" && args[1] == "/1.0/instances/builder?project=hacocoon" {
			return host.Result{Stdout: `{"expanded_devices":{"root":{"type":"disk","path":"/"}}}`}, nil
		}
		return host.Result{}, errors.New("unexpected call")
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.verifyBuilderHasNoNIC(context.Background(), "builder"); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyBuilderHasNoNICFailsClosed(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) == 2 && args[0] == "query" && args[1] == "/1.0/instances/builder?project=hacocoon" {
			return host.Result{Stdout: `{"expanded_devices":{"eth0":{"type":"nic","network":"unexpected"}}}`}, nil
		}
		return host.Result{}, errors.New("unexpected call")
	}}
	provider, _ := NewSandboxProvider(New(runner))
	if err := provider.verifyBuilderHasNoNIC(context.Background(), "builder"); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
}

func TestVerifySeedImageSetRequiresExactDigest(t *testing.T) {
	want := seedbuild.ImageIdentity{
		Reference: "docker.io/library/node:24",
		Digest:    "sha256:" + testFingerprintA,
	}
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 6 && args[0] == "exec" && strings.Contains(strings.Join(args, " "), "nerdctl images") {
			return host.Result{Stdout: "docker.io/library/node\t24\tsha256:" + testFingerprintA + "\n"}, nil
		}
		return host.Result{}, errors.New("unexpected call")
	}}
	provider, _ := NewSandboxProvider(New(runner))
	if err := provider.verifySeedImageSet(context.Background(), "builder", []seedbuild.ImageIdentity{want}); err != nil {
		t.Fatal(err)
	}

	want.Digest = "sha256:" + testFingerprintB
	if err := provider.verifySeedImageSet(context.Background(), "builder", []seedbuild.ImageIdentity{want}); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
}

func TestSeedFingerprintRequiresImmutableSHA256(t *testing.T) {
	if got, err := seedFingerprint(core.BaseRevision("sha256:" + testFingerprintA)); err != nil || got != testFingerprintA {
		t.Fatalf("got=%q err=%v", got, err)
	}
	for _, bad := range []core.BaseRevision{"latest", "sha256:abc", "sha512:" + testFingerprintA} {
		if _, err := seedFingerprint(bad); err == nil {
			t.Fatalf("revision=%q expected error", bad)
		}
	}
}

func TestConfigureNestedOCIBuilderUsesOnlyManagedUnprivilegedSettings(t *testing.T) {
	runner := &fakeRunner{}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	if err := provider.configureNestedOCIInstance(context.Background(), "haco-seed-build-test"); err != nil {
		t.Fatal(err)
	}
	for key, value := range nestedOCIConfig {
		want := "config set haco-seed-build-test " + key + "=" + value
		seen := false
		for _, call := range runner.calls {
			if strings.Contains(strings.Join(call.args, " "), want) {
				seen = true
				break
			}
		}
		if !seen {
			t.Fatalf("missing builder config %q: %#v", want, runner.calls)
		}
	}
	for _, call := range runner.calls {
		if strings.Contains(strings.Join(call.args, " "), "security.privileged=true") {
			t.Fatalf("builder unexpectedly privileged: %#v", call)
		}
	}
}
