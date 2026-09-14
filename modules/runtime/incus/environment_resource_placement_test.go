package incus

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func environmentPlacementFixture() (string, []core.EnvironmentRuntimeAttachment) {
	instance := "env-" + strings.Repeat("a", 32)
	r := core.PersistentResource{ID: "env-data:" + strings.Repeat("b", 32), Owner: strings.Repeat("c", 32), Kind: CacheResourceKind, State: "ready", NativeRef: "pool/haco-persistent-" + strings.Repeat("c", 32), EnvironmentInstance: instance}
	a := core.EnvironmentAttachment{Key: "go", Target: "/root/.cache/go-build", Resource: r.Ref(), Origin: core.ResourceGeneration{Name: "go", Kind: CacheResourceKind, Compatibility: strings.Repeat("d", 64), Epoch: strings.Repeat("e", 32)}}
	return instance, []core.EnvironmentRuntimeAttachment{{Attachment: a, Resource: r}}
}

func TestEnvironmentDataBindingsRejectInvalidPlacementAndOwnership(t *testing.T) {
	instance, areas := environmentPlacementFixture()
	digest, err := environmentDataBinding(instance, areas)
	if err != nil || len(digest) != 64 {
		t.Fatal(digest, err)
	}
	for _, target := range []string{"/", "/root", "/home/user", "/workspace", "/workspace/repo/.git/cache", "/etc/test", "/proc/cache", "/root/.ssh/cache", "/root/.cache/../.ssh", "/root/.config/cache", "/root/.cache\\foo", "/root/.cache\nfoo"} {
		t.Run(target, func(t *testing.T) {
			_, areas := environmentPlacementFixture()
			areas[0].Attachment.Target = target
			if _, err := environmentDataBinding(instance, areas); err == nil {
				t.Fatal("unsafe target accepted")
			}
		})
	}
	for _, mutate := range []func(*core.EnvironmentRuntimeAttachment){
		func(a *core.EnvironmentRuntimeAttachment) {
			a.Resource.EnvironmentInstance = "env-" + strings.Repeat("f", 32)
		},
		func(a *core.EnvironmentRuntimeAttachment) { a.Resource.SourceOnly = true },
		func(a *core.EnvironmentRuntimeAttachment) { a.Resource.State = "creating" },
		func(a *core.EnvironmentRuntimeAttachment) { a.Resource.Kind = OCIStoreKind },
		func(a *core.EnvironmentRuntimeAttachment) { a.Resource.Owner = strings.Repeat("f", 32) },
		func(a *core.EnvironmentRuntimeAttachment) { a.Resource.CopyCompleted = true },
		func(a *core.EnvironmentRuntimeAttachment) { a.Resource.NativeRef = "pool/foreign" },
	} {
		_, areas := environmentPlacementFixture()
		mutate(&areas[0])
		if _, err := environmentDataBinding(instance, areas); err == nil {
			t.Fatal("invalid ownership accepted")
		}
	}
	areas[0].Attachment.Target = "/home/developer/.cache/go-build"
	changed, err := environmentDataBinding(instance, areas)
	if err != nil || changed == digest {
		t.Fatal("placement not bound", err)
	}
}

func TestEnvironmentDataDevicesRequireCompleteExactBinding(t *testing.T) {
	instance, areas := environmentPlacementFixture()
	digest, _ := environmentDataBinding(instance, areas)
	for _, scenario := range []string{"valid", "missing", "extra", "empty-extra", "path", "source", "shift", "overlap-parent", "overlap-child", "owner", "digest", "reference-only", "before-attach"} {
		t.Run(scenario, func(t *testing.T) {
			name := environmentDataDevicePrefix + "go"
			config := environmentDataConfiguration{Config: map[string]string{environmentInstanceKey: instance, environmentDataKey: digest}, Devices: map[string]map[string]string{name: environmentDataDevice(areas[0]), "root": {"type": "disk", "pool": "pool", "path": "/"}}}
			switch scenario {
			case "missing":
				delete(config.Devices, name)
			case "extra":
				config.Devices[environmentDataDevicePrefix+"foreign"] = maps.Clone(config.Devices[name])
			case "empty-extra":
				config.Devices[environmentDataDevicePrefix+"foreign"] = map[string]string{}
			case "path":
				config.Devices[name]["path"] = "/root/.ssh"
			case "source":
				config.Devices[name]["source"] = "foreign"
			case "shift":
				config.Devices[name]["shift"] = "true"
			case "overlap-parent":
				config.Devices["other"] = map[string]string{"type": "disk", "path": "/root"}
			case "overlap-child":
				config.Devices["other"] = map[string]string{"type": "disk", "path": "/root/.cache/go-build/nested"}
			case "owner":
				config.Config[environmentInstanceKey] = "foreign"
			case "digest":
				config.Config[environmentDataKey] = ""
			}
			boundAreas, boundInstance, boundDigest := areas, instance, digest
			if scenario == "reference-only" {
				boundAreas, boundInstance, boundDigest = nil, "", ""
			}
			err := verifyEnvironmentDataDevices(config, boundInstance, boundDigest, boundAreas, nil, scenario != "before-attach")
			if (err == nil) != (scenario == "valid") {
				t.Fatal(scenario, err)
			}
		})
	}
}

func TestEnvironmentDataNativeParentAndExclusiveUse(t *testing.T) {
	instance, areas := environmentPlacementFixture()
	missingParent := areas[0].Resource
	missingParent.EnvironmentInstance = ""
	if _, _, err := managedResourceVolume(missingParent); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal("native disposable volume without parent accepted", err)
	}
	digest, _ := environmentDataBinding(instance, areas)
	for _, scenario := range []string{"valid", "foreign-parent", "missing-parent", "foreign-use", "two-users", "missing-volume", "lost-config"} {
		t.Run(scenario, func(t *testing.T) {
			volume := persistentVolumeObservation{Name: strings.TrimPrefix(areas[0].Resource.NativeRef, "pool/"), Type: "custom", ContentType: "filesystem", Config: persistentResourceConfig(areas[0].Resource), UsedBy: []string{"/1.0/instances/haco-demo?project=hacocoon"}}
			config := environmentDataConfiguration{Config: map[string]string{environmentInstanceKey: instance, environmentDataKey: digest}, Devices: map[string]map[string]string{environmentDataDevicePrefix + "go": environmentDataDevice(areas[0])}}
			switch scenario {
			case "foreign-parent":
				volume.Config[environmentInstanceKey] = "env-" + strings.Repeat("f", 32)
			case "missing-parent":
				delete(volume.Config, environmentInstanceKey)
			case "foreign-use":
				volume.UsedBy[0] = "/1.0/instances/haco-other?project=hacocoon"
			case "two-users":
				volume.UsedBy = append(volume.UsedBy, volume.UsedBy[0])
			}
			p, _ := NewSandboxProvider(New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				var value any
				if len(args) == 2 && args[0] == "query" && args[1] == "/1.0/instances/haco-demo?project=hacocoon" {
					if scenario == "lost-config" {
						return host.Result{}, errors.New("unavailable")
					}
					value = map[string]any{"config": config.Config, "devices": config.Devices, "expanded_config": config.Config, "expanded_devices": config.Devices}
				} else if args[0] == "query" {
					volumes := []persistentVolumeObservation{volume}
					if scenario == "missing-volume" {
						volumes = nil
					}
					value = volumes
				} else {
					t.Fatal("unexpected mutation", args)
				}
				data, _ := json.Marshal(value)
				return host.Result{Stdout: string(data)}, nil
			}}))
			err := p.verifyEnvironmentResources(context.Background(), "haco-demo", core.EnvironmentResourceBinding{InstanceID: instance, Attachments: areas}, core.EnvironmentRunning)
			if (err == nil) != (scenario == "valid") {
				t.Fatal(err)
			}
		})
	}
	for _, raw := range []string{"https://foreign/1.0/instances/haco-demo?project=hacocoon", "/1.0/instances/haco-demo?project=hacocoon&project=other", "/1.0/instances/haco-demo?project=hacocoon#ignored", "/1.0/instances/haco-demo?project=other", "/1.0/instances/haco-demo?project=hacocoon&extra=yes"} {
		if environmentDataUsedBy(raw, "hacocoon", "haco-demo") {
			t.Fatal(raw)
		}
	}
}
