package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestCompletedHostCopyRecoveryFailsClosedAndRestoresExactGuard(t *testing.T) {
	for _, mode := range []string{"frozen", "stopped", "running", "already-restored", "inherited", "unconfirmed", "foreign-consumer", "foreign-role", "wrong-marker-owner", "legacy-journal", "malformed", "extra-field", "guard-changed", "unknown-state", "resume-failed", "clear-failed", "missing-target"} {
		t.Run(mode, func(t *testing.T) {
			source := core.PersistentResource{ID: "oci-source:host", Kind: OCIStoreKind, Owner: strings.Repeat("a", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("a", 32), SourceOnly: true, State: "ready"}
			target := core.PersistentResource{ID: "oci:copy", Kind: OCIStoreKind, Owner: strings.Repeat("b", 32), NativeRef: "pool/haco-persistent-" + strings.Repeat("b", 32), State: "creating", CopySource: source.Ref(), CopyCompleted: true}
			volume := func(r core.PersistentResource) persistentVolumeObservation {
				only := "false"
				if r.SourceOnly {
					only = "true"
				}
				return persistentVolumeObservation{Name: "haco-persistent-" + r.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": r.Owner, "user.hacocoon.resource": r.ID, "user.hacocoon.kind": r.Kind, "user.hacocoon.source-only": only}}
			}
			sv, tv := volume(source), volume(target)
			sv.UsedBy = []string{"/1.0/instances/haco-host?project=hacocoon"}
			journal := hostOCICopyJournal{Version: 1, Owner: target.Owner, Autostart: "true", ExpandedAutostart: "true"}
			if mode == "inherited" {
				journal.Autostart = ""
			}
			if mode == "legacy-journal" {
				journal.Version = 0
			}
			if mode == "wrong-marker-owner" {
				journal.Owner = source.Owner
			}
			raw, _ := json.Marshal(journal)
			marker := string(raw)
			if mode == "malformed" {
				marker = "{"
			}
			if mode == "extra-field" {
				marker = strings.TrimSuffix(marker, "}") + `,"extra":true}`
			}
			i := hostOCICopyInstance{Name: trustedHostName, StatusCode: 110, Config: map[string]string{trustedHostRoleKey: trustedHostRoleValue, hostOCIStoreKey: source.Owner, hostOCICopyKey: marker, "boot.autostart": "false"}, LocalConfig: map[string]string{trustedHostRoleKey: trustedHostRoleValue, hostOCICopyKey: marker, "boot.autostart": "false"}, Devices: map[string]map[string]string{"oci": {"type": "disk", "pool": "pool", "source": sv.Name, "path": OCIStorePath}}}
			switch mode {
			case "stopped":
				i.StatusCode = 102
			case "running":
				i.StatusCode = 103
			case "already-restored":
				i.StatusCode = 103
				delete(i.Config, hostOCICopyKey)
				delete(i.LocalConfig, hostOCICopyKey)
				i.Config["boot.autostart"] = "true"
				i.LocalConfig["boot.autostart"] = "true"
			case "unconfirmed":
				target.CopyCompleted = false
			case "foreign-consumer":
				sv.UsedBy = []string{"/1.0/instances/foreign?project=hacocoon"}
			case "foreign-role":
				delete(i.LocalConfig, trustedHostRoleKey)
			case "guard-changed":
				i.Config["boot.autostart"] = "true"
			case "unknown-state":
				i.StatusCode = 999
			}
			starts, patches := 0, 0
			encode := func(v any) host.Result { data, _ := json.Marshal(v); return host.Result{Stdout: string(data)} }
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" {
					t.Fatal(name)
				}
				switch {
				case args[0] == "query" && strings.Contains(args[1], "/volumes/custom?"):
					if mode == "missing-target" {
						return encode([]persistentVolumeObservation{sv}), nil
					}
					return encode([]persistentVolumeObservation{sv, tv}), nil
				case args[0] == "query" && strings.Contains(args[1], "/instances/"):
					return encode(i), nil
				case args[0] == "start":
					starts++
					if mode == "resume-failed" {
						return host.Result{ExitCode: 1}, nil
					}
					i.StatusCode = 103
					return host.Result{}, nil
				case args[0] == "query" && args[1] == "-X" && args[2] == "PATCH":
					patches++
					if mode == "clear-failed" {
						return host.Result{ExitCode: 1}, nil
					}
					var update struct {
						Config map[string]*string `json:"config"`
					}
					if json.Unmarshal([]byte(args[len(args)-1]), &update) != nil {
						t.Fatal(args)
					}
					for key, value := range update.Config {
						if value == nil {
							delete(i.Config, key)
							delete(i.LocalConfig, key)
							if key == "boot.autostart" {
								i.Config[key] = "true"
							}
						} else {
							i.Config[key] = *value
							i.LocalConfig[key] = *value
						}
					}
					return host.Result{}, nil
				default:
					t.Fatal("unexpected recovery command", args)
					return host.Result{}, nil
				}
			}}
			err := (&PersistentResourceBackend{Runtime: New(runner)}).RecoverCompletedCopy(context.Background(), source, target)
			success := mode == "frozen" || mode == "stopped" || mode == "running" || mode == "already-restored" || mode == "inherited"
			if (err == nil) != success {
				t.Fatalf("%s: %v", mode, err)
			}
			if success {
				if i.StatusCode != 103 || i.Config[hostOCICopyKey] != "" || i.Config["boot.autostart"] != "true" {
					t.Fatal("restoration incomplete")
				}
				if mode == "inherited" && i.LocalConfig["boot.autostart"] != "" {
					t.Fatal("profile autostart overridden")
				}
				if (mode == "running" || mode == "already-restored") && starts != 0 {
					t.Fatal("running Host started again")
				}
			} else if mode != "resume-failed" && mode != "clear-failed" && (starts != 0 || patches != 0) {
				t.Fatal("unsafe state mutated")
			}
		})
	}
}
