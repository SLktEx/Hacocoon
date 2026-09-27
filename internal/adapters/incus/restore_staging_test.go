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
	for _, mode := range []string{"ok", "foreign-owner", "host-device", "running", "duplicate", "delete-retained", "binding-drift"} {
		t.Run(mode, func(t *testing.T) {
			p := baseSnapshotFixture()
			sourceConfig := p.config()
			sourceConfig["volatile.last_state.ready"] = "true"
			sourceConfig["volatile.base_image"] = strings.Repeat("b", 64)
			root := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": "pool"}}
			source := snapshotInstanceObservation{Name: p.target(), Type: "container", Status: "Stopped", Config: sourceConfig, ExpandedConfig: sourceConfig, Devices: root, ExpandedDevices: root}
			instances := []snapshotInstanceObservation{source}
			deleted := false
			var r *Runtime
			binding := restoreBinding{Version: 1, Project: "hacocoon", Owner: strings.Repeat("e", 32)}
			r = New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if mode == "binding-drift" {
					t.Fatal("provider called with invalid binding")
				}
				if args[0] == "delete" {
					deleted = true
					if mode != "delete-retained" {
						instances = []snapshotInstanceObservation{}
					}
					return host.Result{}, nil
				}
				if args[0] != "query" || len(args) != 2 {
					t.Fatal("migration cleanup attempted creation", args)
				}
				raw, _ := json.Marshal(instances)
				return host.Result{Stdout: string(raw)}, nil
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
			binding.Source = src
			cfg := binding.config(true)
			target := snapshotInstanceObservation{Name: binding.target(), Type: "container", Status: "Stopped", Config: cfg, ExpandedConfig: cfg, Devices: root, ExpandedDevices: root}
			switch mode {
			case "foreign-owner":
				target.Config["user.hacocoon.owner"] = "foreign"
			case "host-device":
				target.Devices["host"] = map[string]string{"type": "disk", "source": "/", "path": "/host"}
			case "running":
				target.Status = "Running"
			}
			instances = []snapshotInstanceObservation{target}
			if mode == "duplicate" {
				instances = append(instances, target)
			}
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
