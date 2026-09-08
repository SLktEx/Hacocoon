package incus

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnvironmentCreationRetainsBaseBeforeRuntimeMutation(t *testing.T) {
	for _, kind := range []string{"base", "sandbox"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if len(args) > 1 && args[0] == "image" && args[1] == "info" {
					return host.Result{Stdout: `{"fingerprint":"` + testFingerprintA + `"}`}, nil
				}
				if len(args) > 1 && args[0] == "profile" && args[1] == "show" {
					return rootProfileResult(), nil
				}
				if len(args) > 0 && (args[0] == "init" || args[0] == "start" || args[0] == "delete") {
					t.Fatal("Environment mutated after failed Base retention", args)
				}
				return host.Result{}, nil
			}}
			provider, err := NewSandboxProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			provider.ConfigureBaseRetention(func(_ context.Context, base core.BaseRef, scope, source string) (core.BaseAsset, error) {
				calls++
				if base.Name != defaultBaseName || base.Revision != core.BaseRevision("sha256:"+testFingerprintA) || scope != "hacocoon/default" || source != "images:"+testFingerprintA {
					t.Fatal(base, scope, source)
				}
				return core.BaseAsset{}, core.ErrRecoveryRequired
			})
			spec := core.EnvironmentRuntimeSpec{InstanceID: testEnvironmentInstance, Name: "demo", WorkspacePath: "/tmp/work"}
			if kind == "base" {
				_, err = provider.BaseProvider.CreateEnvironment(context.Background(), spec)
			} else {
				_, err = provider.CreateEnvironment(context.Background(), spec)
			}
			if calls != 1 || !errors.Is(err, core.ErrRuntimeUnavailable) || errors.Is(err, core.ErrRecoveryRequired) {
				t.Fatal("independent Base failure held Environment ownership", calls, err)
			}
		})
	}
}

func TestRetainedBaseRequiresExactReadyReceipt(t *testing.T) {
	for _, mode := range []string{"ready", "wrong-base", "wrong-provider", "wrong-scope", "created"} {
		t.Run(mode, func(t *testing.T) {
			provider, err := NewBaseProvider(New(&fakeRunner{}))
			if err != nil {
				t.Fatal(err)
			}
			resolved := resolvedBase{ref: baseSnapshotFixture().Base, pinnedSource: "local:" + strings.Repeat("b", 64)}
			provider.ConfigureBaseRetention(func(_ context.Context, base core.BaseRef, scope, source string) (core.BaseAsset, error) {
				if source != resolved.pinnedSource {
					t.Fatal("effective source changed")
				}
				a := readyBaseReceipt(base, scope, source)
				switch mode {
				case "wrong-base":
					a.Base.Revision = "other"
				case "wrong-provider":
					a.Provider = "other"
				case "wrong-scope":
					a.Scope = "other/pool"
				case "created":
					a.State = "created"
				}
				return a, nil
			})
			if err := provider.retainResolvedBase(context.Background(), resolved, "pool"); (err == nil) != (mode == "ready") {
				t.Fatal(err)
			}
		})
	}
}

func readyBaseReceipt(base core.BaseRef, scope, source string) core.BaseAsset {
	project, pool, _ := strings.Cut(scope, "/")
	owner := strings.Repeat("c", 32)
	binding, _ := json.Marshal(baseAssetBinding{Version: 1, Project: project, Pool: pool, Source: source})
	return core.BaseAsset{ID: "base-" + owner, Owner: owner, Base: base, Provider: "incus", Scope: scope, NativeRef: "instance/haco-base-" + owner, Binding: string(binding), State: "ready"}
}
