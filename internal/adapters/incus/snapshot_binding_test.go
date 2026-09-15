package incus

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestSnapshotBindingRejectsDriftBeforeProviderAccess(t *testing.T) {
	root, _ := rootfsFixture()
	base := baseSnapshotFixture()
	volume := snapshotVolumePlan{Pool: "pool", Source: "haco-work-member", SourceOwner: strings.Repeat("b", 32), SourceKind: "work", SourceID: "repo", SourceInstance: root.Source, SourceInstanceID: root.SourceInstanceID, Owner: strings.Repeat("c", 32), Role: "workspace:member"}
	bindings := []snapshotBinding{{Version: 1, Project: "hacocoon", Rootfs: &root}, {Version: 1, Project: "hacocoon", Base: &base}, {Version: 1, Project: "hacocoon", Volume: &volume}}
	for _, b := range bindings {
		r := New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
			t.Fatal("unexpected provider access")
			return host.Result{}, nil
		}})
		original, err := r.snapshotComponent(b)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []string{"owner", "role", "ref", "unknown", "duplicate", "project", "version", "empty", "trailing", "source", "state"} {
			t.Run(original.Role+"/"+mode, func(t *testing.T) {
				c := original
				source := core.SnapshotSource{Environment: core.Environment{RuntimeRef: root.Source, Base: &base.Base}, InstanceID: root.SourceInstanceID}
				switch mode {
				case "owner":
					c.Owner = strings.Repeat("f", 32)
				case "role":
					c.Role = "unknown"
				case "ref":
					c.NativeRef += "-foreign"
				case "unknown":
					c.Binding = strings.Replace(c.Binding, `"version":1`, `"unknown":true,"version":1`, 1)
				case "duplicate":
					c.Binding = strings.Replace(c.Binding, `"version":1`, `"version":0,"version":1`, 1)
				case "project":
					c.Binding = strings.Replace(c.Binding, `"project":"hacocoon"`, `"project":"foreign"`, 1)
				case "version":
					c.Binding = strings.Replace(c.Binding, `"version":1`, `"version":2`, 1)
				case "empty":
					c.Binding = ""
				case "trailing":
					c.Binding += " "
				case "source":
					source.InstanceID = "env-ffffffffffffffffffffffffffffffff"
					source.Environment.Base = nil
				case "state":
					c.State = "created"
				}
				if err := r.createSnapshotComponent(context.Background(), source, c); err == nil {
					t.Fatal("drift accepted")
				}
				if mode != "source" && mode != "state" {
					c.State = "created"
					if r.verifySnapshotComponent(context.Background(), c) == nil || r.deleteSnapshotComponent(context.Background(), c) == nil {
						t.Fatal("drift accepted on retry")
					}
				}
			})
		}
	}
}
func TestSnapshotBindingReloadCleansOnlyExactSavedTarget(t *testing.T) {
	root, _ := rootfsFixture()
	base := baseSnapshotFixture()
	for _, b := range []snapshotBinding{{Version: 1, Project: "hacocoon", Rootfs: &root}, {Version: 1, Project: "hacocoon", Base: &base}} {
		r := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
			if len(args) != 2 || args[0] != "query" || args[1] != "/1.0/instances?project=hacocoon&recursion=1" {
				t.Fatal(args)
			}
			return host.Result{Stdout: "[]"}, nil
		}})
		c, err := r.snapshotComponent(b)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(c)
		if err != nil {
			t.Fatal(err)
		}
		var reopened core.SnapshotComponent
		if json.Unmarshal(raw, &reopened) != nil {
			t.Fatal("reload")
		}
		if err := r.deleteSnapshotComponent(context.Background(), reopened); err != nil {
			t.Fatal(err)
		}
	}
}
