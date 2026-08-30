package incus

import (
	"context"
	"errors"
	"reflect"
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

func TestVerifySeedImageSetInspectsExactIdentity(t *testing.T) {
	want := seedbuild.ImageIdentity{
		Reference: "docker.io/library/node:24",
		Digest:    "sha256:" + testFingerprintA,
	}
	wantArgs := []string{
		"exec", "builder", "--project", "hacocoon", "--",
		"nerdctl", "image", "inspect", want.String(),
	}
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" || !reflect.DeepEqual(args, wantArgs) {
			t.Fatalf("unexpected call: %s %#v", name, args)
		}
		return host.Result{Stdout: `[{"Id":"` + testFingerprintA + `"}]`}, nil
	}}
	provider, _ := NewSandboxProvider(New(runner))
	if err := provider.verifySeedImageSet(context.Background(), "builder", []seedbuild.ImageIdentity{want}); err != nil {
		t.Fatal(err)
	}
}

func TestVerifySeedImageSetFailsWhenExactIdentityIsMissing(t *testing.T) {
	want := seedbuild.ImageIdentity{
		Reference: "docker.io/library/node:24",
		Digest:    "sha256:" + testFingerprintB,
	}
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" || !strings.Contains(strings.Join(args, " "), "nerdctl image inspect "+want.String()) {
			t.Fatalf("unexpected call: %s %#v", name, args)
		}
		return host.Result{ExitCode: 1, Stderr: "no such image"}, errors.New("exit status 1")
	}}
	provider, _ := NewSandboxProvider(New(runner))
	if err := provider.verifySeedImageSet(context.Background(), "builder", []seedbuild.ImageIdentity{want}); err == nil {
		t.Fatal("expected exact identity verification failure")
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
