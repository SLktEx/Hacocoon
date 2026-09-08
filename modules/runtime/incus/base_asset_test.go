package incus

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestBaseAssetBackendPlanCreateAndCacheIndependentVerify(t *testing.T) {
	for _, mode := range []string{"ok", "lost-reply", "foreign-owner", "profile", "running", "wrong-revision", "host-device", "not-btrfs", "truncated", "missing"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			base := baseSnapshotFixture().Base
			owner := strings.Repeat("a", 32)
			p := baseStorageIdentity{Kind: "base", Pool: "pool", Owner: owner, Base: base}
			calls, creates, observations := 0, 0, 0
			runtime := New(&fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				calls++
				if name != "incus" {
					t.Fatal(name)
				}
				if args[0] == "init" {
					creates++
					if !reflect.DeepEqual(args[:8], []string{"init", "images:" + strings.Repeat("b", 64), p.target(), "--project", "hacocoon", "--no-profiles", "--storage", "pool"}) {
						t.Fatal(args)
					}
					config := map[string]string{}
					for i := 8; i < len(args); i += 2 {
						if args[i] != "--config" {
							t.Fatal(args)
						}
						k, v, ok := strings.Cut(args[i+1], "=")
						if !ok {
							t.Fatal(args)
						}
						config[k] = v
					}
					if !reflect.DeepEqual(config, p.config()) {
						t.Fatal(config)
					}
					if mode == "lost-reply" {
						return host.Result{ExitCode: 1}, errors.New("lost reply")
					}
					return host.Result{}, nil
				}
				if reflect.DeepEqual(args, []string{"query", "/1.0/storage-pools/pool"}) {
					driver := "btrfs"
					if mode == "not-btrfs" {
						driver = "dir"
					}
					raw, _ := json.Marshal(map[string]string{"name": "pool", "driver": driver})
					return host.Result{Stdout: string(raw)}, nil
				}
				if !reflect.DeepEqual(args, []string{"query", "/1.0/instances?project=hacocoon&recursion=1"}) {
					t.Fatal("verification touched image cache or unexpected provider endpoint", args)
				}
				observations++
				config := p.config()
				config["volatile.base_image"] = strings.Repeat("b", 64)
				devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": "pool"}}
				item := snapshotInstanceObservation{Name: p.target(), Type: "container", Status: "Stopped", Profiles: []string{}, Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices}
				switch mode {
				case "foreign-owner":
					config["user.hacocoon.owner"] = strings.Repeat("c", 32)
				case "profile":
					item.Profiles = []string{"default"}
				case "running":
					item.Status = "Running"
				case "wrong-revision":
					config["volatile.base_image"] = strings.Repeat("c", 64)
				case "host-device":
					devices["host"] = map[string]string{"type": "disk", "source": "/", "path": "/host"}
				}
				items := []snapshotInstanceObservation{item}
				if mode == "missing" {
					items = nil
				}
				raw, _ := json.Marshal(items)
				return host.Result{Stdout: string(raw), StdoutTruncated: mode == "truncated"}, nil
			}})
			provider, err := NewBaseProvider(runtime)
			if err != nil {
				t.Fatal(err)
			}
			b := &BaseAssetBackend{Provider: provider}
			native, binding, err := b.Plan(ctx, base, "hacocoon/pool", owner)
			if mode == "not-btrfs" {
				if err == nil || creates != 0 {
					t.Fatal(err, creates)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			a := core.BaseAsset{ID: "base-" + owner, Owner: owner, Base: base, Provider: "incus", Scope: "hacocoon/pool", NativeRef: native, Binding: binding, State: "planned"}
			before := calls
			err = b.Create(ctx, a)
			if calls != before+2 || observations != 0 {
				t.Fatal("operation after create before receipt", calls, observations)
			}
			if mode == "lost-reply" {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			a.State = "created"
			err = b.Verify(ctx, a)
			if (err == nil) != (mode == "ok") {
				t.Fatal(err)
			}
		})
	}
}

func TestBaseAssetBackendRejectsBindingDriftBeforeProviderAccess(t *testing.T) {
	p := baseSnapshotFixture()
	binding, _ := json.Marshal(baseAssetBinding{Version: 1, Project: "hacocoon", Pool: p.Pool, Source: "images:" + strings.Repeat("b", 64)})
	initial := core.BaseAsset{ID: "base-" + p.Owner, Owner: p.Owner, Base: p.Base, Provider: "incus", Scope: "hacocoon/pool", NativeRef: "instance/haco-base-" + p.Owner, Binding: string(binding), State: "planned"}
	for _, mode := range []string{"provider", "scope", "native", "owner", "id", "base", "unknown-field", "duplicate-field", "source", "state"} {
		t.Run(mode, func(t *testing.T) {
			a := initial
			switch mode {
			case "provider":
				a.Provider = "other"
			case "scope":
				a.Scope = "other/pool"
			case "native":
				a.NativeRef = "instance/haco-host"
			case "owner":
				a.Owner = strings.Repeat("c", 32)
			case "id":
				a.ID = "base-" + strings.Repeat("c", 32)
			case "base":
				a.Base.Revision = core.BaseRevision("sha256:" + strings.Repeat("c", 64))
			case "unknown-field":
				a.Binding = strings.TrimSuffix(a.Binding, "}") + `,"authority":true}`
			case "duplicate-field":
				a.Binding = strings.Replace(a.Binding, `"version":1`, `"version":1,"version":1`, 1)
			case "source":
				a.Binding = strings.Replace(a.Binding, "images:", "-option:", 1)
			case "state":
				a.State = "ready"
			}
			provider, err := NewBaseProvider(New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
				t.Fatal("provider called with drifted plan")
				return host.Result{}, nil
			}}))
			if err != nil {
				t.Fatal(err)
			}
			if err := (&BaseAssetBackend{Provider: provider}).Create(context.Background(), a); err == nil {
				t.Fatal("accepted drift")
			}
		})
	}
}

func TestBaseAssetPlanPreservesResolvedLocalSource(t *testing.T) {
	p := baseSnapshotFixture()
	provider, err := NewBaseProvider(New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if !reflect.DeepEqual(args, []string{"query", "/1.0/storage-pools/pool"}) {
			t.Fatal("resolved source was looked up again", args)
		}
		return host.Result{Stdout: `{"name":"pool","driver":"btrfs"}`}, nil
	}}))
	if err != nil {
		t.Fatal(err)
	}
	backend := &BaseAssetBackend{Provider: provider, PinnedSource: "local:" + strings.Repeat("b", 64)}
	_, binding, err := backend.Plan(context.Background(), p.Base, "hacocoon/pool", p.Owner)
	if err != nil {
		t.Fatal(err)
	}
	var plan baseAssetBinding
	if err := json.Unmarshal([]byte(binding), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Source != backend.PinnedSource {
		t.Fatal("effective source substituted", plan.Source)
	}
	backend.PinnedSource = "local:" + strings.Repeat("c", 64)
	if _, _, err := backend.Plan(context.Background(), p.Base, "hacocoon/pool", p.Owner); err == nil {
		t.Fatal("accepted another revision")
	}
}
