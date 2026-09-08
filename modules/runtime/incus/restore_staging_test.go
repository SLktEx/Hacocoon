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

func TestRestoreStagingOwnershipAndIndependentCleanup(t *testing.T) {
	for _, mode := range []string{"ok", "lost-reply", "foreign-owner", "host-device", "running", "duplicate", "delete-retained", "binding-drift"} {
		t.Run(mode, func(t *testing.T) {
			p := baseSnapshotFixture()
			sourceConfig := p.config()
			sourceConfig["volatile.base_image"] = strings.Repeat("b", 64)
			root := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": "pool"}}
			source := snapshotInstanceObservation{Name: p.target(), Type: "container", Status: "Stopped", Config: sourceConfig, ExpandedConfig: sourceConfig, Devices: root, ExpandedDevices: root}
			instances := []snapshotInstanceObservation{source}
			created, receipt, deleted := false, false, false
			r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if mode == "binding-drift" {
					t.Fatal("provider called with invalid binding")
				}
				if created && !receipt {
					t.Fatal("fallible operation before create receipt")
				}
				if args[0] == "delete" {
					deleted = true
					if mode != "delete-retained" {
						instances = []snapshotInstanceObservation{}
					}
					return host.Result{}, nil
				}
				switch args[1] {
				case "/1.0/instances?project=hacocoon&recursion=1":
					raw, _ := json.Marshal(instances)
					return host.Result{Stdout: string(raw)}, nil
				case "/1.0/storage-pools/pool":
					return host.Result{Stdout: `{"name":"pool","driver":"btrfs"}`}, nil
				case "-X":
					var req struct {
						Name      string
						Config    map[string]string
						Devices   map[string]map[string]string
						Source    map[string]any
						Profiles  []string
						Ephemeral bool
					}
					if json.Unmarshal([]byte(args[6]), &req) != nil || req.Source["type"] != "copy" || req.Source["source"] != p.target() || req.Source["instance_only"] != true || req.Source["live"] != false || req.Config["user.hacocoon.kind"] != "restore-staging" || req.Config["user.hacocoon.owner"] == p.Owner || req.Profiles == nil || len(req.Profiles) != 0 || req.Ephemeral {
						t.Fatal("unsafe copy", req)
					}
					target := snapshotInstanceObservation{Name: req.Name, Type: "container", Status: "Stopped", Config: req.Config, ExpandedConfig: req.Config, Devices: req.Devices, ExpandedDevices: req.Devices}
					switch mode {
					case "foreign-owner":
						target.Config["user.hacocoon.owner"] = "foreign"
					case "host-device":
						target.Devices["host"] = map[string]string{"type": "disk", "source": "/", "path": "/host"}
					case "running":
						target.Status = "Running"
					}
					// The original is absent: verification/cleanup must use only the staged target.
					instances = []snapshotInstanceObservation{target}
					if mode == "duplicate" {
						instances = append(instances, target)
					}
					created = true
					if mode == "lost-reply" {
						return host.Result{ExitCode: 1}, nil
					}
					return host.Result{}, nil
				default:
					t.Fatal("unexpected provider call", args)
					return host.Result{}, nil
				}
			}})
			src, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: "hacocoon", Base: &p})
			if err != nil {
				t.Fatal(err)
			}
			src.State = "verified"
			c, err := r.restoreComponent(restoreBinding{Version: 1, Project: "hacocoon", Owner: strings.Repeat("e", 32), Source: src})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "binding-drift" {
				c.Binding = strings.Replace(c.Binding, `"version":1`, `"version":1,"extra":true`, 1)
			}
			err = r.CreateRestoreComponent(context.Background(), core.Snapshot{State: "ready", Components: []core.SnapshotComponent{src}}, c)
			if mode == "binding-drift" {
				if err == nil {
					t.Fatal("invalid binding accepted")
				}
				return
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
			receipt = true
			c.State = "created"
			err = r.VerifyRestoreComponent(context.Background(), c)
			if mode != "ok" && mode != "delete-retained" {
				if err == nil {
					t.Fatal("unsafe target accepted")
				}
				if r.DeleteRestoreComponent(context.Background(), c) == nil || deleted {
					t.Fatal("unsafe target deleted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			err = r.DeleteRestoreComponent(context.Background(), c)
			if mode == "delete-retained" {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal("absence assumed", err)
				}
			} else if err != nil || !deleted {
				t.Fatal(err, deleted)
			}
		})
	}
}
