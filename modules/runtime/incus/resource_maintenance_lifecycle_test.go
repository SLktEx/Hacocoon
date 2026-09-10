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

func TestMaintenanceCreationRecordsBeforePreparingAndAttaching(t *testing.T) {
	for _, mode := range []string{"ok", "preparation-failure", "tooling-failure", "startup-failure"} {
		t.Run(mode, func(t *testing.T) {
			work, _ := core.NewTemporaryWorkspace()
			resource := core.PersistentResource{ID: "oci:retained", Owner: strings.Repeat("a", 32), Kind: OCIStoreKind, State: "ready", NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32)}
			initialized, recorded, started, prepared, attached, maintenance, guardCreated := false, false, false, false, false, false, false
			config := map[string]string{environmentInstanceKey: testEnvironmentInstance}
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if initialized && !recorded {
					t.Fatal("fallible work before receipt", args)
				}
				if args[0] == "delete" {
					t.Fatal("provider discarded caller-owned receipt")
				}
				if args[0] == "init" {
					initialized = true
				}
				if args[0] == "start" {
					if attached {
						t.Fatal("retained data exposed to ordinary boot")
					}
					started = true
				}
				if args[0] == "exec" {
					script := args[len(args)-1]
					switch script {
					case persistentOCIConfiguration:
						t.Fatal("ordinary retained daemon setup invoked")
					case resourceMaintenancePreparation:
						if !started || attached {
							t.Fatal("preparation order")
						}
						if mode == "preparation-failure" {
							return host.Result{ExitCode: 43}, errors.New("preparation failed")
						}
						prepared = true
					case containerdMaintenanceStart:
						if !prepared || !attached {
							t.Fatal("metadata startup order")
						}
						maintenance = true
						if mode == "startup-failure" {
							return host.Result{ExitCode: 42}, nil
						}
					}
				}
				if len(args) > 4 && args[0] == "config" && args[1] == "device" && args[2] == "add" && args[4] == "persistent-resource" {
					if !prepared {
						t.Fatal("retained mount before preparation")
					}
					attached = true
				}
				if args[0] == "query" && strings.HasPrefix(args[1], "/1.0/storage-pools/") {
					used := []string{}
					if attached {
						used = []string{"/1.0/instances/haco-demo?project=hacocoon"}
					}
					data, _ := json.Marshal([]persistentVolumeObservation{{Name: "haco-persistent-" + resource.Owner, Type: "custom", ContentType: "filesystem", UsedBy: used, Config: map[string]string{"user.hacocoon.owner": resource.Owner, "user.hacocoon.resource": resource.ID, "user.hacocoon.kind": resource.Kind}}})
					return host.Result{Stdout: string(data)}, nil
				}
				if args[0] == "query" && strings.HasPrefix(args[1], "/1.0/instances/haco-demo?") {
					devices := map[string]map[string]string{"persistent-resource": {"type": "disk", "pool": "pool", "source": "haco-persistent-" + resource.Owner, "path": OCIStorePath}}
					if !attached {
						devices = map[string]map[string]string{}
					}
					data, _ := json.Marshal(snapshotInstanceObservation{Name: "haco-demo", Type: "container", Status: "Running", Profiles: []string{}, Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices})
					return host.Result{Stdout: string(data)}, nil
				}
				if len(args) >= 2 && args[0] == "image" && args[1] == "info" {
					return host.Result{Stdout: `{"fingerprint":"` + sandboxTestFingerprint + `"}`}, nil
				}
				if len(args) > 6 && args[2] == "nft" && args[6] == routedSandboxGuardTable("haco-demo") {
					if args[3] == "list" && !guardCreated {
						return host.Result{Stderr: "No such file or directory"}, errors.New("absent guard")
					}
					if args[3] == "add" && args[4] == "table" {
						guardCreated = true
					}
				}
				if result, ok := sandboxNetworkResult(args); ok {
					return result, nil
				}
				if len(args) >= 3 && args[0] == "profile" && args[1] == "show" && args[2] == "default" {
					return rootProfileResult(), nil
				}
				if len(args) >= 4 && args[0] == "config" && args[1] == "set" {
					parts := strings.SplitN(args[3], "=", 2)
					config[parts[0]] = parts[1]
				}
				if len(args) >= 4 && args[0] == "config" && args[1] == "get" {
					return host.Result{Stdout: config[args[3]]}, nil
				}
				return host.Result{}, nil
			}}
			provider, err := NewSandboxProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			if mode == "tooling-failure" {
				provider.ConfigureMaintenanceTooling(func(context.Context) (string, func() error, error) {
					if !recorded || !prepared || attached {
						t.Fatal("tooling violated receipt/preparation/mount order")
					}
					return "", nil, core.ErrRuntimeUnavailable
				})
			}
			created, err := provider.CreateEnvironmentWithReceipt(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", InstanceID: testEnvironmentInstance, TemporaryWorkspace: true, WorkspacePath: work.Path, PersistentResource: resource, ResourceMaintenance: true}, func(created core.EnvironmentRuntime) error {
				if !initialized || recorded || created.Ref != "haco-demo" {
					t.Fatal("invalid receipt")
				}
				recorded = true
				return nil
			})
			if (mode == "ok") != (err == nil) {
				t.Fatal(err)
			}
			if !recorded || created.Ref != "haco-demo" || !guardCreated {
				t.Fatal("creation lost ownership or source guard")
			}
			if mode == "preparation-failure" || mode == "tooling-failure" {
				if attached || maintenance {
					t.Fatal("failed preparation attached data")
				}
			} else if !attached || !maintenance {
				t.Fatal("maintenance never started", err)
			}
		})
	}
}
func TestMaintenanceCreationRejectsNonTemporaryAndSavedSources(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("invalid maintenance used native authority")
		return host.Result{}, nil
	}}
	provider, _ := NewSandboxProvider(New(runner))
	record := func(core.EnvironmentRuntime) error { t.Fatal("unexpected receipt"); return nil }
	work, _ := core.NewTemporaryWorkspace()
	resource := core.PersistentResource{ID: "oci:retained", Owner: strings.Repeat("a", 32), Kind: OCIStoreKind, State: "ready"}
	for _, mode := range []string{"ordinary", "readonly", "source-only", "missing-resource"} {
		spec := core.EnvironmentRuntimeSpec{Name: "demo", InstanceID: testEnvironmentInstance, TemporaryWorkspace: true, WorkspacePath: work.Path, PersistentResource: resource, ResourceMaintenance: true}
		switch mode {
		case "ordinary":
			spec.TemporaryWorkspace = false
		case "readonly":
			spec.ReadOnly = true
		case "source-only":
			spec.PersistentResource.SourceOnly = true
		case "missing-resource":
			spec.PersistentResource = core.PersistentResource{}
		}
		if _, err := provider.CreateEnvironmentWithReceipt(context.Background(), spec, record); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(mode, err)
		}
	}
	if _, err := provider.CreateEnvironmentFromSnapshot(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", InstanceID: testEnvironmentInstance, WorkspacePath: "/workspace", ResourceMaintenance: true}, core.Snapshot{State: "ready"}, record); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}
