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

func TestSnapshotPublishesOrdinaryImageWithAtomicOwnership(t *testing.T) {
	root, _ := rootfsFixture()
	plan := snapshotImagePlan{Name: "snap-" + strings.Repeat("e", 32), Owner: strings.Repeat("f", 32), Rootfs: root}
	config := root.config()
	devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": root.Pool}}
	saved := snapshotInstanceObservation{Name: root.target(), Type: "container", Status: "Stopped", Config: config, ExpandedConfig: config, Devices: devices, ExpandedDevices: devices}
	published := false
	runtime := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) == 2 && strings.Contains(args[1], "/instances?") {
			raw, _ := json.Marshal([]snapshotInstanceObservation{saved})
			return host.Result{Stdout: string(raw)}, nil
		}
		if len(args) > 3 && args[1] == "-X" && args[2] == "GET" && strings.Contains(args[3], "/images/aliases?") {
			return host.Result{Stdout: "[]"}, nil
		}
		if len(args) > 5 && args[2] == "POST" {
			var data struct {
				Properties map[string]string
				Source     map[string]string
				Aliases    []map[string]string
			}
			if json.Unmarshal([]byte(args[5]), &data) != nil || data.Properties["user.hacocoon.snapshot-owner"] != plan.Owner || data.Properties["user.hacocoon.kind"] != "base-image" || data.Source["name"] != root.target() || data.Aliases[0]["name"] != builtBasePrefix+plan.Name {
				t.Fatal(args)
			}
			published = true
			return host.Result{}, nil
		}
		t.Fatal("unexpected provider call", args)
		return host.Result{}, nil
	}})
	if err := runtime.createSnapshotImage(context.Background(), plan); err != nil || !published {
		t.Fatal(err, published)
	}
}
func TestSnapshotImageCleanupUsesOwnershipInventoryWithoutAlias(t *testing.T) {
	for _, foreign := range []bool{false, true} {
		root, _ := rootfsFixture()
		plan := snapshotImagePlan{Name: "saved", Owner: strings.Repeat("f", 32), Rootfs: root}
		exists := true
		deletes := 0
		runtime := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
			if len(args) > 3 && args[2] == "GET" && strings.Contains(args[3], "images?project=") {
				images := []baseImage{}
				if exists {
					item := baseImage{Fingerprint: strings.Repeat("a", 64), Type: "container", Properties: map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": plan.Name, "user.hacocoon.build-instance": root.SourceInstanceID, "user.hacocoon.snapshot-owner": plan.Owner}}
					if foreign {
						item.Properties["user.hacocoon.base-name"] = "foreign"
					}
					images = append(images, item)
				}
				raw, _ := json.Marshal(images)
				return host.Result{Stdout: string(raw)}, nil
			}
			if len(args) > 3 && args[2] == "DELETE" {
				deletes++
				exists = false
				return host.Result{}, nil
			}
			t.Fatal(args)
			return host.Result{}, nil
		}})
		err := runtime.deleteSnapshotImage(context.Background(), plan)
		if foreign {
			if !errors.Is(err, core.ErrCapabilityStale) || deletes != 0 {
				t.Fatal(err, deletes)
			}
		} else if err != nil || deletes != 1 || exists {
			t.Fatal(err, deletes, exists)
		}
	}
}
