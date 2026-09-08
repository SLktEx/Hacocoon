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

func TestHostAreaCopyPausesExactOwnerAndRestoresOnlyAfterCompletion(t *testing.T) {
	for _, scenario := range []string{"ok", "inherited-autostart", "foreign-consumer", "extra-consumer", "wrong-path", "foreign-host", "already-paused", "pending", "pause-failed", "freeze-unconfirmed", "copy-failed", "copy-exit", "copy-truncated", "resume-failed", "clear-failed"} {
		t.Run(scenario, func(t *testing.T) {
			source := core.PersistentResource{ID: "oci-source:host", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), SourceOnly: true}
			target := core.PersistentResource{ID: "oci:copy", Kind: OCIStoreKind, Owner: strings.Repeat("b", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("b", 32), CopySource: source.Ref()}
			volume := persistentVolumeObservation{Name: "haco-persistent-" + source.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": source.Owner, "user.hacocoon.resource": source.ID, "user.hacocoon.kind": source.Kind, "user.hacocoon.source-only": "true"}, UsedBy: []string{"/1.0/instances/haco-host?project=hacocoon"}}
			instance := hostOCICopyInstance{Name: trustedHostName, StatusCode: 103, Config: map[string]string{trustedHostRoleKey: trustedHostRoleValue, "boot.autostart": "true"}, Devices: map[string]map[string]string{"oci": {"type": "disk", "pool": "pool", "source": volume.Name, "path": OCIStorePath}}}
			instance.LocalConfig = map[string]string{"boot.autostart": "true"}
			switch scenario {
			case "inherited-autostart":
				delete(instance.LocalConfig, "boot.autostart")
			case "foreign-consumer":
				volume.UsedBy[0] = "/1.0/instances/foreign?project=hacocoon"
			case "extra-consumer":
				volume.UsedBy = append(volume.UsedBy, volume.UsedBy[0])
			case "wrong-path":
				instance.Devices["oci"]["path"] = "/root"
			case "foreign-host":
				instance.Config[trustedHostRoleKey] = "other"
			case "already-paused":
				instance.StatusCode = 110
			case "pending":
				instance.Config[hostOCICopyKey] = "other-operation"
			}
			copies, pauses, resumes := 0, 0, 0
			encode := func(v any) host.Result { data, _ := json.Marshal(v); return host.Result{Stdout: string(data)} }
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal("unexpected executable")
				}
				switch {
				case args[0] == "query" && args[1] == "/1.0/storage-pools/pool":
					return host.Result{Stdout: `{"name":"pool","driver":"btrfs"}`}, nil
				case args[0] == "query" && strings.HasPrefix(args[1], "/1.0/storage-pools/pool/volumes/custom?"):
					return encode([]persistentVolumeObservation{volume}), nil
				case args[0] == "query" && strings.HasPrefix(args[1], "/1.0/instances/haco-host?"):
					return encode(instance), nil
				case args[0] == "query" && args[2] == "PATCH":
					var patch struct {
						Config map[string]*string `json:"config"`
					}
					if json.Unmarshal([]byte(args[len(args)-1]), &patch) != nil {
						t.Fatal("bad patch")
					}
					if patch.Config[hostOCICopyKey] == nil && scenario == "clear-failed" {
						return host.Result{}, errors.New("lost clear reply")
					}
					for key, value := range patch.Config {
						if value == nil {
							delete(instance.Config, key)
							delete(instance.LocalConfig, key)
							if scenario == "inherited-autostart" && key == "boot.autostart" {
								instance.Config[key] = "true"
							}
						} else {
							instance.Config[key] = *value
							instance.LocalConfig[key] = *value
						}
					}
					return host.Result{}, nil
				case args[0] == "pause":
					pauses++
					if instance.Config[hostOCICopyKey] == "" || instance.Config["boot.autostart"] != "false" {
						t.Fatal("pause before durable restart guard")
					}
					if scenario == "pause-failed" {
						return host.Result{}, errors.New("lost pause reply")
					}
					if scenario != "freeze-unconfirmed" {
						instance.StatusCode = 110
					}
					return host.Result{}, nil
				case args[0] == "query" && args[2] == "POST":
					copies++
					if instance.StatusCode != 110 || instance.Config["boot.autostart"] != "false" {
						t.Fatal("copy while writers can run")
					}
					if scenario == "copy-failed" {
						return host.Result{}, errors.New("lost copy reply")
					}
					if scenario == "copy-exit" {
						return host.Result{ExitCode: 1}, nil
					}
					if scenario == "copy-truncated" {
						return host.Result{StdoutTruncated: true}, nil
					}
					return host.Result{}, nil
				case args[0] == "start":
					resumes++
					if copies != 1 {
						t.Fatal("resumed without completed copy")
					}
					if scenario == "resume-failed" {
						return host.Result{}, errors.New("lost resume reply")
					}
					instance.StatusCode = 103
					return host.Result{}, nil
				default:
					t.Fatalf("unexpected command %v", args)
					return host.Result{}, nil
				}
			}}
			err := (&PersistentResourceBackend{Runtime: New(runner)}).Copy(context.Background(), source, target)
			if scenario == "ok" || scenario == "inherited-autostart" {
				if scenario == "inherited-autostart" && instance.LocalConfig["boot.autostart"] != "" {
					t.Fatal("profile autostart replaced by local override")
				}
				if err != nil || copies != 1 || pauses != 1 || resumes != 1 || instance.Config[hostOCICopyKey] != "" || instance.Config["boot.autostart"] != "true" {
					t.Fatalf("copy/restoration: %v %+v", err, instance)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe copy succeeded")
				}
				if strings.HasPrefix(scenario, "copy-") && (resumes != 0 || instance.Config[hostOCICopyKey] == "" || instance.Config["boot.autostart"] != "false") {
					t.Fatal("uncertain copy resumed writers or lost journal")
				}
			}
		})
	}
}

func TestHostEntryCannotRestartAnInterruptedCopy(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		return host.Result{Stdout: `{"owner":"interrupted"}`}, nil
	}}
	err := New(runner).ensureTrustedHostRunning(context.Background(), "STOPPED")
	if !errors.Is(err, core.ErrRecoveryRequired) || len(runner.calls) != 1 {
		t.Fatal("Host entry bypassed pending copy")
	}
}
