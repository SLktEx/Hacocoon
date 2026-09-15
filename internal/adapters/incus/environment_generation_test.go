package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestGenerationCopyRequiresExactStoppedOrdinaryConsumer(t *testing.T) {
	for _, mode := range []string{"valid", "running", "absent", "foreign", "autostart", "host", "two-consumers", "wrong-project", "escaped-path", "other-device", "no-device", "ordinary-target"} {
		t.Run(mode, func(t *testing.T) {
			instance, areas := environmentPlacementFixture()
			source := areas[0].Resource
			target := core.PersistentResource{ID: "generation:" + strings.Repeat("d", 32), Owner: strings.Repeat("e", 32), Kind: CacheResourceKind, NativeRef: "pool/haco-persistent-" + strings.Repeat("e", 32), SourceOnly: true, CopySource: source.Ref()}
			ref := "haco-producer"
			if mode == "host" {
				ref = trustedHostName
			}
			observed := &persistentVolumeObservation{UsedBy: []string{"/1.0/instances/" + ref + "?project=" + defaultProject}}
			config := map[string]string{environmentInstanceKey: instance, environmentDataKey: strings.Repeat("f", 64), "boot.autostart": "false"}
			devices := map[string]map[string]string{environmentDataDevicePrefix + "go": environmentDataDevice(areas[0])}
			switch mode {
			case "two-consumers":
				observed.UsedBy = append(observed.UsedBy, observed.UsedBy[0])
			case "wrong-project":
				observed.UsedBy[0] = "/1.0/instances/" + ref + "?project=foreign"
			case "escaped-path":
				observed.UsedBy[0] = "/1.0/instances/%68aco-producer?project=" + defaultProject
			case "autostart":
				config["boot.autostart"] = "true"
			case "other-device":
				devices["other"] = devices[environmentDataDevicePrefix+"go"]
				delete(devices, environmentDataDevicePrefix+"go")
			case "no-device":
				devices = map[string]map[string]string{}
			case "ordinary-target":
				target.SourceOnly = false
			}
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				switch args[0] {
				case "config":
					value := instance
					if mode == "foreign" {
						value = "env-" + strings.Repeat("f", 32)
					}
					return host.Result{Stdout: value}, nil
				case "query":
					data, err := json.Marshal(map[string]any{"config": config, "expanded_config": config, "devices": devices, "expanded_devices": devices})
					if err != nil {
						t.Fatal(err)
					}
					return host.Result{Stdout: string(data)}, nil
				case "list":
					state := "STOPPED"
					if mode == "running" {
						state = "RUNNING"
					}
					text := ref + "," + state + "\n"
					if mode == "absent" {
						text = ""
					}
					return host.Result{Stdout: text}, nil
				default:
					t.Fatalf("unexpected mutation: %v", args)
					return host.Result{}, nil
				}
			}}
			err := (&PersistentResourceBackend{Runtime: New(runner)}).verifyEnvironmentGenerationSource(context.Background(), source, target, observed)
			if (err == nil) != (mode == "valid") {
				t.Fatal(mode, err)
			}
		})
	}
}
