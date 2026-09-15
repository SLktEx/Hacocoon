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

func baseSnapshotFixture() snapshotBasePlan {
	return snapshotBasePlan{Pool: "pool", Owner: strings.Repeat("a", 32), Base: core.BaseRef{Name: "haco/ubuntu-26.04", Revision: core.BaseRevision("sha256:" + strings.Repeat("b", 64))}}
}
func TestSnapshotBaseCreatePinsLocalRevision(t *testing.T) {
	for _, mode := range []string{"ok", "bad-plan", "missing", "truncated", "wrong-image", "wrong-type", "wrong-pool", "not-btrfs", "lost-reply"} {
		t.Run(mode, func(t *testing.T) {
			p := baseSnapshotFixture()
			if mode == "bad-plan" {
				p.Pool = "../foreign"
			}
			calls, posts := 0, 0
			r := New(&fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				calls++
				if name != "incus" {
					t.Fatal(name)
				}
				if strings.HasPrefix(args[1], "/1.0/images/") {
					if !reflect.DeepEqual(args, []string{"query", "/1.0/images/" + strings.Repeat("b", 64) + "?project=hacocoon"}) {
						t.Fatal(args)
					}
					if mode == "missing" {
						return host.Result{ExitCode: 1}, nil
					}
					fingerprint, kind := strings.Repeat("b", 64), "container"
					if mode == "wrong-image" {
						fingerprint = strings.Repeat("c", 64)
					}
					if mode == "wrong-type" {
						kind = "virtual-machine"
					}
					data, _ := json.Marshal(map[string]string{"fingerprint": fingerprint, "type": kind})
					return host.Result{Stdout: string(data), StdoutTruncated: mode == "truncated"}, nil
				}
				if args[1] == "/1.0/storage-pools/pool" {
					pool, driver := "pool", "btrfs"
					if mode == "wrong-pool" {
						pool = "foreign"
					}
					if mode == "not-btrfs" {
						driver = "dir"
					}
					data, _ := json.Marshal(map[string]string{"name": pool, "driver": driver})
					return host.Result{Stdout: string(data)}, nil
				}
				posts++
				if !reflect.DeepEqual(args[:6], []string{"query", "-X", "POST", "--wait", "/1.0/instances?project=hacocoon", "--data"}) {
					t.Fatal(args)
				}
				var req struct {
					Name      string
					Type      string
					Ephemeral bool
					Profiles  []string
					Config    map[string]string
					Devices   map[string]map[string]string
					Source    map[string]string
				}
				if json.Unmarshal([]byte(args[6]), &req) != nil {
					t.Fatal("invalid request")
				}
				if req.Name != p.target() || req.Type != "container" || req.Ephemeral || req.Profiles == nil || len(req.Profiles) != 0 || !reflect.DeepEqual(req.Config, p.config()) || !reflect.DeepEqual(req.Devices, map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": "pool"}}) || !reflect.DeepEqual(req.Source, map[string]string{"type": "image", "fingerprint": strings.Repeat("b", 64)}) {
					t.Fatal(req)
				}
				if mode == "lost-reply" {
					return host.Result{}, errors.New("lost")
				}
				return host.Result{}, nil
			}})
			err := r.createSnapshotBase(context.Background(), p)
			switch mode {
			case "ok":
				if err != nil || calls != 3 || posts != 1 {
					t.Fatal(err, calls, posts)
				}
			case "lost-reply":
				if !errors.Is(err, core.ErrRecoveryRequired) || calls != 3 || posts != 1 {
					t.Fatal(err, calls, posts)
				}
			default:
				if err == nil || posts != 0 || (mode == "bad-plan" && calls != 0) {
					t.Fatal(err, calls, posts)
				}
			}
		})
	}
}
func TestSnapshotBaseCleanupRequiresExactOwnedStoppedInstance(t *testing.T) {
	for _, mode := range []string{"ok", "already-absent", "foreign-owner", "foreign-revision", "running", "profile", "ephemeral", "nic", "root-option", "hook", "duplicate", "malformed", "truncated", "delete-failed", "still-present", "absence-failed"} {
		t.Run(mode, func(t *testing.T) {
			p := baseSnapshotFixture()
			config := p.config()
			config["volatile.base_image"] = strings.Repeat("b", 64)
			config["image.description"] = "untrusted image metadata"
			devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": "pool"}}
			item := snapshotInstanceObservation{Name: p.target(), Type: "container", Status: "Stopped", Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices, Profiles: []string{}}
			switch mode {
			case "foreign-owner":
				config["user.hacocoon.owner"] = strings.Repeat("c", 32)
			case "foreign-revision":
				config["volatile.base_image"] = strings.Repeat("c", 64)
			case "running":
				item.Status = "Running"
			case "profile":
				item.Profiles = []string{"default"}
			case "ephemeral":
				item.Ephemeral = true
			case "nic":
				devices["eth0"] = map[string]string{"type": "nic"}
			case "root-option":
				devices["root"]["source"] = "/foreign"
			case "hook":
				config["raw.lxc"] = "synthetic-hook"
			}
			observations, deletes := 0, 0
			r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] == "delete" {
					deletes++
					if !reflect.DeepEqual(args, []string{"delete", p.target(), "--project", "hacocoon"}) {
						t.Fatal(args)
					}
					if mode == "delete-failed" {
						return host.Result{ExitCode: 1}, nil
					}
					return host.Result{}, nil
				}
				observations++
				if mode == "malformed" {
					return host.Result{Stdout: "null"}, nil
				}
				if mode == "absence-failed" && observations > 1 {
					return host.Result{}, errors.New("lost inventory")
				}
				items := []snapshotInstanceObservation{item}
				if mode == "already-absent" || (observations > 1 && mode != "still-present") {
					items = []snapshotInstanceObservation{}
				}
				if mode == "duplicate" {
					items = append(items, item)
				}
				data, _ := json.Marshal(items)
				return host.Result{Stdout: string(data), StdoutTruncated: mode == "truncated"}, nil
			}})
			err := r.deleteSnapshotBase(context.Background(), p)
			allowed := mode == "ok" || mode == "already-absent"
			if (err == nil) != allowed {
				t.Fatal(err)
			}
			wantDelete := mode == "ok" || mode == "delete-failed" || mode == "still-present" || mode == "absence-failed"
			if (deletes == 1) != wantDelete {
				t.Fatal("unsafe deletion", deletes)
			}
		})
	}
}

func TestSnapshotBaseCopiesRetainedMaterialWithoutCacheAndSurvivesSourceDeletion(t *testing.T) {
	for _, mode := range []string{"ok", "lost-reply", "foreign-owner"} {
		t.Run(mode, func(t *testing.T) {
			p := baseSnapshotFixture()
			asset := readyBaseReceipt(p.Base, "hacocoon/pool", "local:"+strings.Repeat("b", 64))
			p.Asset = &asset
			source := baseStorageIdentity{Kind: "base", Pool: p.Pool, Owner: asset.Owner, Base: p.Base}
			observe := func(identity baseStorageIdentity) snapshotInstanceObservation {
				config := identity.config()
				config["volatile.base_image"] = strings.Repeat("b", 64)
				devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": p.Pool}}
				return snapshotInstanceObservation{Name: identity.target(), Type: "container", Status: "Stopped", Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices}
			}
			sourceItem := observe(source)
			if mode == "foreign-owner" {
				sourceItem.Config["user.hacocoon.owner"] = "foreign"
			}
			items := []snapshotInstanceObservation{sourceItem}
			created, receipt, deleted := false, false, false
			r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if created && !receipt {
					t.Fatal("provider call before create receipt", args)
				}
				switch {
				case args[0] == "query" && args[1] == "/1.0/instances?project=hacocoon&recursion=1":
					raw, _ := json.Marshal(items)
					return host.Result{Stdout: string(raw)}, nil
				case args[0] == "query" && args[1] == "/1.0/storage-pools/pool":
					return host.Result{Stdout: `{"name":"pool","driver":"btrfs"}`}, nil
				case args[0] == "query" && args[1] == "-X":
					var req struct {
						Name   string
						Config map[string]string
						Source map[string]any
					}
					if json.Unmarshal([]byte(args[6]), &req) != nil || req.Name != p.target() || !reflect.DeepEqual(req.Config, p.config()) || !reflect.DeepEqual(req.Source, map[string]any{"type": "copy", "source": source.target(), "project": "hacocoon", "instance_only": true, "live": false}) {
						t.Fatal("unexpected copy", req)
					}
					created = true
					// Only the independent target remains after the source is deleted.
					items = []snapshotInstanceObservation{observe(p.storageIdentity())}
					if mode == "lost-reply" {
						return host.Result{ExitCode: 1}, nil
					}
					return host.Result{}, nil
				case args[0] == "delete":
					if args[1] != p.target() {
						t.Fatal("deleted source", args)
					}
					items = []snapshotInstanceObservation{}
					deleted = true
					return host.Result{}, nil
				default:
					t.Fatal("unexpected cache access or provider operation", args)
					return host.Result{}, nil
				}
			}})
			c, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: "hacocoon", Base: &p})
			if err != nil {
				t.Fatal(err)
			}
			err = r.createSnapshotComponent(context.Background(), core.SnapshotSource{Environment: core.Environment{Base: &p.Base}}, c)
			if mode == "foreign-owner" {
				if err == nil || created {
					t.Fatal(err, created)
				}
				return
			}
			if mode == "lost-reply" {
				if !errors.Is(err, core.ErrRecoveryRequired) {
					t.Fatal(err)
				}
				return
			}
			if err != nil || !created {
				t.Fatal(err, created)
			}
			receipt = true
			c.State = "created"
			if err = r.verifySnapshotComponent(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			if err = r.deleteSnapshotComponent(context.Background(), c); err != nil || !deleted {
				t.Fatal(err, deleted)
			}
		})
	}
}
