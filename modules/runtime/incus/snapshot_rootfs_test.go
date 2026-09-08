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

func rootfsFixture() (snapshotRootfsPlan, snapshotInstanceObservation) {
	p := snapshotRootfsPlan{Pool: "pool", Source: "haco-source", SourceInstanceID: testEnvironmentInstance, Owner: strings.Repeat("a", 32)}
	config := map[string]string{environmentInstanceKey: p.SourceInstanceID, managedEnvironmentMarkerKey: managedEnvironmentMarkerValue, "environment.TEST_TOKEN": "synthetic-secret", "raw.lxc": "synthetic-hook", "boot.autostart": "true", "volatile.idmap.current": "[]"}
	devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": p.Pool}, "workspace": {"type": "disk", "path": "/workspace", "source": "/synthetic-work"}, "eth0": {"type": "nic", "nictype": "bridged", "parent": "synthetic"}, "control": {"type": "proxy", "listen": "unix:/tmp/synthetic", "connect": "unix:/tmp/host-synthetic"}}
	return p, snapshotInstanceObservation{Name: p.Source, Type: "container", Status: "Stopped", Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices, Profiles: []string{"default"}}
}
func TestSnapshotRootfsClearsInheritedAuthorityBeforeCopy(t *testing.T) {
	for _, mode := range []string{"ok", "foreign", "running", "missing", "duplicate", "malformed", "truncated", "wrong-pool", "not-btrfs", "bad-idmap", "lost-reply"} {
		t.Run(mode, func(t *testing.T) {
			p, source := rootfsFixture()
			posts := 0
			switch mode {
			case "foreign":
				source.Config[environmentInstanceKey] = "env-22222222222222222222222222222222"
			case "running":
				source.Status = "Running"
			case "wrong-pool":
				source.Devices["root"]["pool"] = "foreign"
			case "bad-idmap":
				source.Config["volatile.idmap.current"] = "null"
			}
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || args[0] != "query" {
					t.Fatal(name, args)
				}
				if args[1] == "-X" {
					posts++
					var req struct {
						Name      string
						Config    map[string]string
						Devices   map[string]map[string]string
						Profiles  []string
						Ephemeral bool
						Source    map[string]any
					}
					if err := json.Unmarshal([]byte(args[len(args)-1]), &req); err != nil {
						t.Fatal(err)
					}
					if strings.Contains(args[len(args)-1], "synthetic-secret") || strings.Contains(args[len(args)-1], "synthetic-hook") {
						t.Fatal("source config exported")
					}
					if req.Name != p.target() || req.Profiles == nil || len(req.Profiles) != 0 || req.Ephemeral {
						t.Fatal(req)
					}
					for k, v := range p.config() {
						if req.Config[k] != v {
							t.Fatal("unsafe target config", req.Config)
						}
					}
					if req.Config["volatile.idmap.current"] != "[]" {
						t.Fatal("lost idmap")
					}
					for name, device := range req.Devices {
						if name == "root" {
							if !reflect.DeepEqual(device, map[string]string{"type": "disk", "path": "/", "pool": "pool"}) {
								t.Fatal(device)
							}
						} else if !reflect.DeepEqual(device, map[string]string{"type": "none"}) {
							t.Fatal("inherited device", device)
						}
					}
					if !reflect.DeepEqual(req.Source, map[string]any{"type": "copy", "source": p.Source, "project": "hacocoon", "instance_only": true, "live": false}) {
						t.Fatal(req.Source)
					}
					if mode == "lost-reply" {
						return host.Result{}, errors.New("lost reply")
					}
					return host.Result{}, nil
				}
				if args[1] == "/1.0/storage-pools/pool" {
					driver := "btrfs"
					if mode == "not-btrfs" {
						driver = "dir"
					}
					return host.Result{Stdout: `{"name":"pool","driver":"` + driver + `"}`}, nil
				}
				if mode == "malformed" {
					return host.Result{Stdout: "null"}, nil
				}
				list := []snapshotInstanceObservation{source}
				if mode == "missing" {
					list = []snapshotInstanceObservation{}
				}
				if mode == "duplicate" {
					list = append(list, source)
				}
				data, _ := json.Marshal(list)
				return host.Result{Stdout: string(data), StdoutTruncated: mode == "truncated"}, nil
			}}
			err := New(runner).createSnapshotRootfs(context.Background(), p)
			if mode == "ok" {
				if err != nil || posts != 1 {
					t.Fatal(err, posts)
				}
			} else if mode == "lost-reply" {
				if !errors.Is(err, core.ErrRecoveryRequired) || posts != 1 {
					t.Fatal(err, posts)
				}
			} else if err == nil || posts != 0 {
				t.Fatal("unsafe copy", err, posts)
			}
		})
	}
}
func TestSnapshotRootfsCleanupRejectsChangedOwnershipAndAuthority(t *testing.T) {
	for _, mode := range []string{"ok", "foreign", "autostart", "profile", "nic", "secret", "root-option", "running", "delete-failed", "still-present", "absence-failed"} {
		t.Run(mode, func(t *testing.T) {
			p, _ := rootfsFixture()
			deletes := 0
			config := p.config()
			devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": p.Pool}, "eth0": {"type": "none"}}
			target := snapshotInstanceObservation{Name: p.target(), Type: "container", Status: "Stopped", Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices, Profiles: []string{}}
			switch mode {
			case "foreign":
				config["user.hacocoon.owner"] = "foreign"
			case "autostart":
				config["boot.autostart"] = "true"
			case "profile":
				target.Profiles = []string{"default"}
			case "nic":
				devices["eth0"] = map[string]string{"type": "nic"}
			case "secret":
				config["environment.TEST_TOKEN"] = "synthetic"
			case "root-option":
				devices["root"]["shift"] = "true"
			case "running":
				target.Status = "Running"
			}
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] == "delete" {
					deletes++
					if !reflect.DeepEqual(args, []string{"delete", p.target(), "--project", "hacocoon"}) {
						t.Fatal(args)
					}
					if mode == "delete-failed" {
						return host.Result{}, errors.New("failed")
					}
					return host.Result{}, nil
				}
				if deletes > 0 {
					if mode == "absence-failed" {
						return host.Result{}, errors.New("unknown")
					}
					if mode != "still-present" {
						return host.Result{Stdout: "[]"}, nil
					}
				}
				data, _ := json.Marshal([]snapshotInstanceObservation{target})
				return host.Result{Stdout: string(data)}, nil
			}}
			err := New(runner).deleteSnapshotRootfs(context.Background(), p)
			if mode == "ok" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unsafe cleanup")
			}
			if mode != "ok" && mode != "delete-failed" && mode != "still-present" && mode != "absence-failed" && deletes != 0 {
				t.Fatal("changed target deleted")
			}
		})
	}
}
