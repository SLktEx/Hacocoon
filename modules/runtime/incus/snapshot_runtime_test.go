package incus

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func savedRuntimeFixture() (snapshotRootfsPlan, snapshotInstanceObservation) {
	p, _ := rootfsFixture()
	config := p.config()
	config["volatile.idmap.current"] = "[]"
	config["volatile.last_state.ready"] = "true"
	devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": p.Pool}, "workspace": {"type": "none"}, "persistent-resource": {"type": "none"}}
	return p, snapshotInstanceObservation{Name: p.target(), Type: "container", Status: "Stopped", Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices}
}
func TestSavedRuntimeCopyUsesIndependentRootAndFreshIdentity(t *testing.T) {
	for _, mode := range []string{"ok", "foreign", "running", "profile", "old-generation", "not-btrfs", "lost-reply"} {
		t.Run(mode, func(t *testing.T) {
			p, observed := savedRuntimeFixture()
			posts := 0
			if mode == "foreign" {
				observed.Config["user.hacocoon.owner"] = "foreign"
			}
			if mode == "running" {
				observed.Status = "Running"
			}
			if mode == "profile" {
				observed.Profiles = []string{"old"}
			}
			r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] != "query" {
					t.Fatal("consulted original image/profile", args)
				}
				var value any
				if args[1] == "-X" {
					posts++
					var request struct {
						Name     string
						Profiles []string
						Config   map[string]string
						Devices  map[string]map[string]string
						Source   map[string]any
					}
					if json.Unmarshal([]byte(args[len(args)-1]), &request) != nil {
						t.Fatal("invalid request")
					}
					if request.Name != "haco-new" || len(request.Profiles) != 0 || request.Source["source"] != p.target() || request.Source["instance_only"] != true || request.Source["live"] != false || request.Config[environmentInstanceKey] == p.SourceInstanceID || request.Config["user.hacocoon.owner"] != "" || len(request.Devices) != 3 {
						t.Fatal(request)
					}
					if request.Config["volatile.last_state.ready"] != "false" {
						t.Fatal("saved ready state not reset as boolean")
					}
					if mode == "lost-reply" {
						return host.Result{}, errors.New("lost response")
					}
					return host.Result{}, nil
				}
				if strings.HasPrefix(args[1], "/1.0/instances?") {
					value = []snapshotInstanceObservation{observed}
				} else if args[1] == "/1.0/storage-pools/"+p.Pool {
					driver := "btrfs"
					if mode == "not-btrfs" {
						driver = "dir"
					}
					value = map[string]string{"name": p.Pool, "driver": driver}
				} else {
					t.Fatal("unexpected original dependency", args)
				}
				raw, _ := json.Marshal(value)
				return host.Result{Stdout: string(raw)}, nil
			}})
			config := map[string]string{environmentInstanceKey: "env-" + strings.Repeat("f", 32), managedEnvironmentMarkerKey: managedEnvironmentMarkerValue, "security.privileged": "false", "security.nesting": "false", "boot.autostart": "false"}
			if mode == "old-generation" {
				config[environmentInstanceKey] = p.SourceInstanceID
			}
			_, err := r.copySavedRuntime(context.Background(), p, "haco-new", config)
			if mode == "ok" {
				if err != nil || posts != 1 {
					t.Fatal(err, posts)
				}
			} else {
				if err == nil {
					t.Fatal("unsafe copy accepted")
				}
				if mode != "lost-reply" && posts != 0 {
					t.Fatal("unexpected mutation")
				}
			}
		})
	}
}
func TestSavedRuntimeRecordsBeforeCurrentConfiguration(t *testing.T) {
	for _, mode := range []string{"ok", "receipt", "configuration", "ssh", "ssh-exit", "masks"} {
		t.Run(mode, func(t *testing.T) {
			root, observed := savedRuntimeFixture()
			copied, recorded, renewed, guard := false, false, false, false
			values := map[string]string{}
			masks := map[string]bool{"workspace": true, "persistent-resource": true}
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if copied && !recorded {
					t.Fatal("provider call before ownership receipt", args)
				}
				if args[0] == "image" || (len(args) > 2 && args[0] == "profile" && args[2] == "default") {
					t.Fatal("Base/default dependency", args)
				}
				if args[0] == "delete" {
					t.Fatal("provider duplicated caller cleanup")
				}
				if args[0] == "query" && strings.HasPrefix(args[1], "/1.0/instances?") {
					raw, _ := json.Marshal([]snapshotInstanceObservation{observed})
					return host.Result{Stdout: string(raw)}, nil
				}
				if args[0] == "query" && args[1] == "/1.0/storage-pools/"+root.Pool {
					return host.Result{Stdout: `{"name":"` + root.Pool + `","driver":"btrfs"}`}, nil
				}
				if args[0] == "query" && args[1] == "-X" {
					copied = true
					return host.Result{}, nil
				}
				if len(args) > 6 && args[2] == "nft" && args[6] == routedSandboxGuardTable("haco-demo") {
					if args[3] == "list" && !guard {
						return host.Result{Stderr: "No such file or directory"}, errors.New("missing")
					}
					if args[3] == "add" && args[4] == "table" {
						guard = true
					}
				}
				if len(args) > 3 && args[0] == "config" && args[1] == "device" {
					if args[2] == "remove" {
						if !copied || !recorded || len(args) != 8 || args[5] != "--" || args[6] != "haco-demo" || !masks[args[7]] {
							t.Fatal("unsafe mask removal", args)
						}
						if mode == "masks" {
							return host.Result{ExitCode: 1}, nil
						}
						delete(masks, args[7])
					}
					if args[2] == "add" && len(args) > 4 && masks[args[4]] {
						return host.Result{ExitCode: 1}, errors.New("device already exists")
					}
				}
				if result, ok := sandboxNetworkResult(args); ok {
					return result, nil
				}
				if len(args) >= 4 && args[0] == "config" && args[1] == "set" {
					if mode == "configuration" {
						return host.Result{}, errors.New("configuration failed")
					}
					parts := strings.SplitN(args[3], "=", 2)
					values[parts[0]] = parts[1]
					return host.Result{}, nil
				}
				if len(args) >= 4 && args[0] == "config" && args[1] == "get" {
					return host.Result{Stdout: values[args[3]]}, nil
				}
				if args[0] == "exec" && args[len(args)-1] == freshGuestSSHIdentity {
					renewed = true
					if mode == "ssh-exit" {
						return host.Result{ExitCode: 1}, nil
					}
					if mode == "ssh" {
						return host.Result{}, errors.New("reset failed")
					}
				}
				return host.Result{}, nil
			}}
			p, err := NewSandboxProvider(New(runner))
			if err != nil {
				t.Fatal(err)
			}
			c, err := p.snapshotComponent(snapshotBinding{Version: 1, Project: p.project, Rootfs: &root})
			if err != nil {
				t.Fatal(err)
			}
			c.State = "verified"
			saved := core.Snapshot{State: "ready", Components: []core.SnapshotComponent{c}}
			result, err := p.CreateEnvironmentFromSnapshot(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", InstanceID: "env-" + strings.Repeat("f", 32), WorkspacePath: "/tmp/work"}, saved, func(created core.EnvironmentRuntime) error {
				if !copied || recorded || created.Ref != "haco-demo" || created.Base != nil {
					t.Fatal("bad receipt", created)
				}
				recorded = true
				if mode == "receipt" {
					return errors.New("record failed")
				}
				return nil
			})
			if !copied || !recorded || result.Ref != "haco-demo" {
				t.Fatal("ownership missing", result, err)
			}
			if mode == "ok" {
				if err != nil || !renewed {
					t.Fatal("current configuration incomplete", err)
				}
			} else if err == nil {
				t.Fatal("failure published")
			}
		})
	}
}
func TestSavedRuntimeRejectsMissingGenerationBeforeProvider(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		t.Fatal("provider called")
		return host.Result{}, nil
	}}
	p, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.CreateEnvironmentFromSnapshot(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/work"}, core.Snapshot{State: "ready"}, func(core.EnvironmentRuntime) error { return nil }); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal(err)
	}
}
