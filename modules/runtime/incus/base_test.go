package incus

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const testFingerprintA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const testFingerprintB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func TestBaseProviderListsLogicalNamesWithoutIncusDetails(t *testing.T) {
	t.Setenv(baseConfigEnv, `{"my-dev":"local:mutable-alias"}`)
	provider, err := NewBaseProvider(New(&fakeRunner{}))
	if err != nil {
		t.Fatal(err)
	}
	bases, err := provider.ListBases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []core.BaseInfo{
		{Name: "haco/ubuntu-24.04"},
		{Name: "haco/ubuntu-26.04"},
		{Name: "my-dev"},
	}
	if !reflect.DeepEqual(bases, want) {
		t.Fatalf("bases=%#v want=%#v", bases, want)
	}
}

func TestInspectBaseResolvesImmutableRevision(t *testing.T) {
	t.Setenv(baseConfigEnv, `{"my-dev":"images:mutable-alias"}`)
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if reflect.DeepEqual(args, []string{"image", "info", "images:mutable-alias", "--format", "json"}) {
			return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
		}
		return host.Result{}, errors.New("unexpected call")
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	info, err := provider.InspectBase(context.Background(), "my-dev")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "my-dev" || info.Revision != core.BaseRevision("sha256:"+testFingerprintA) {
		t.Fatalf("info=%#v", info)
	}
}

func TestCreateEnvironmentPinsResolvedFingerprintAndPersistsBase(t *testing.T) {
	t.Setenv(baseConfigEnv, `{"my-dev":"images:mutable-alias"}`)
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "image" && args[1] == "info" {
			return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
		}
		if len(args) >= 2 && args[0] == "profile" && args[1] == "show" {
			return rootProfileResult(), nil
		}
		return host.Result{}, nil
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	created, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/tmp/workspace",
		Base:          "my-dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Base == nil || created.Base.Name != "my-dev" || created.Base.Revision != core.BaseRevision("sha256:"+testFingerprintA) {
		t.Fatalf("created Base=%#v", created.Base)
	}
	pinned := "images:" + testFingerprintA
	seenPinnedInit := false
	for _, call := range runner.calls {
		if len(call.args) >= 3 && call.args[0] == "init" {
			seenPinnedInit = call.args[1] == pinned
			if call.args[1] == "images:mutable-alias" {
				t.Fatalf("mutable alias used at init: %#v", call)
			}
		}
	}
	if !seenPinnedInit {
		t.Fatalf("pinned init missing from calls: %#v", runner.calls)
	}
}

func TestBaseAliasCanMoveWithoutChangingEarlierResolvedRevision(t *testing.T) {
	t.Setenv(baseConfigEnv, `{"my-dev":"images:mutable-alias"}`)
	fingerprint := testFingerprintA
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "image" && args[1] == "info" {
			return host.Result{Stdout: `{"fingerprint":"` + fingerprint + `"}`}, nil
		}
		return host.Result{}, nil
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	first, err := provider.InspectBase(context.Background(), "my-dev")
	if err != nil {
		t.Fatal(err)
	}
	fingerprint = testFingerprintB
	second, err := provider.InspectBase(context.Background(), "my-dev")
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != core.BaseRevision("sha256:"+testFingerprintA) || second.Revision != core.BaseRevision("sha256:"+testFingerprintB) {
		t.Fatalf("first=%#v second=%#v", first, second)
	}
}

func TestCustomBasesCannotOverrideOfficialNamespace(t *testing.T) {
	t.Setenv(baseConfigEnv, `{"haco/ubuntu-26.04":"evil:alias"}`)
	_, err := NewBaseProvider(New(&fakeRunner{}))
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("err=%v want ErrInvalidArgument", err)
	}
}

func TestCustomBaseConfigRejectsAuthorityShapingInputs(t *testing.T) {
	cases := []string{
		`{"../victim":"images:ubuntu/26.04"}`,
		`{"good":"--project"}`,
		`{"good\nHost evil":"images:ubuntu/26.04"}`,
	}
	for _, raw := range cases {
		t.Run(raw, func(t *testing.T) {
			old, existed := os.LookupEnv(baseConfigEnv)
			if err := os.Setenv(baseConfigEnv, raw); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if existed {
					_ = os.Setenv(baseConfigEnv, old)
				} else {
					_ = os.Unsetenv(baseConfigEnv)
				}
			})
			_, err := NewBaseProvider(New(&fakeRunner{}))
			if !errors.Is(err, core.ErrInvalidArgument) {
				t.Fatalf("config %q err=%v want ErrInvalidArgument", raw, err)
			}
		})
	}
}

func TestResolveBaseRejectsMalformedFingerprint(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "image" && args[1] == "info" {
			return host.Result{Stdout: `{"fingerprint":"../../mutable"}`}, nil
		}
		return host.Result{}, nil
	}}
	provider, err := NewBaseProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.InspectBase(context.Background(), defaultBaseName)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v want ErrIncompatibleState", err)
	}
}

func TestPinImageSourceRetainsOnlyRemotePrefix(t *testing.T) {
	if got := pinImageSource("images:ubuntu/26.04", testFingerprintA); got != "images:"+testFingerprintA {
		t.Fatalf("got=%q", got)
	}
	if got := pinImageSource("local-alias", testFingerprintA); got != testFingerprintA {
		t.Fatalf("got=%q", got)
	}
	if strings.Contains(pinImageSource("images:mutable-alias", testFingerprintA), "mutable") {
		t.Fatal("mutable alias survived pinning")
	}
}
