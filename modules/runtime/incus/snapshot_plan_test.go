package incus

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestSnapshotPlanEnumeratesAggregateAndRefusesOmissions(t *testing.T) {
	for _, mode := range []string{"ok", "no-oci", "missing-work", "missing-base", "extra-disk", "duplicate-work", "foreign-owner", "running", "foreign-instance", "missing-volume", "foreign-user", "wrong-image", "missing-image", "readonly-option", "wrong-pool"} {
		t.Run(mode, func(t *testing.T) {
			root, instance := rootfsFixture()
			base := baseSnapshotFixture()
			source := core.SnapshotSource{Environment: core.Environment{RuntimeRef: root.Source, Workspace: core.Workspace{ID: "work", Path: "managed:work"}, Base: &base.Base, PersistentResource: core.PersistentResourceRef{ID: "oci:dev", Owner: strings.Repeat("d", 32)}}, InstanceID: root.SourceInstanceID}
			mounts := []WorkspaceAttachment{{Device: "workspace-one", Pool: "pool", Volume: "haco-work-work-one", Path: "/workspace/one", Owner: strings.Repeat("b", 32), Repository: "one"}, {Device: "workspace-two", Pool: "pool", Volume: "haco-work-work-two", Path: "/workspace/two", Owner: strings.Repeat("c", 32), Repository: "two"}}
			devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": "pool"}, "eth0": {"type": "nic"}}
			volumes := []persistentVolumeObservation{}
			for _, m := range mounts {
				devices[m.Device] = map[string]string{"type": "disk", "pool": m.Pool, "source": m.Volume, "path": m.Path}
				volumes = append(volumes, persistentVolumeObservation{Name: m.Volume, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": m.Owner, "user.hacocoon.role": "work", "user.hacocoon.repository": m.Repository}, UsedBy: []string{"/1.0/instances/" + root.Source + "?project=hacocoon"}})
			}
			devices["persistent-resource"] = map[string]string{"type": "disk", "pool": "pool", "source": "haco-persistent-" + source.Environment.PersistentResource.Owner, "path": OCIStorePath}
			volumes = append(volumes, persistentVolumeObservation{Name: "haco-persistent-" + source.Environment.PersistentResource.Owner, Type: "custom", ContentType: "filesystem", Config: map[string]string{"user.hacocoon.owner": source.Environment.PersistentResource.Owner, "user.hacocoon.resource": "oci:dev", "user.hacocoon.kind": OCIStoreKind, "user.hacocoon.source-only": "false"}})
			instance.Devices, instance.ExpandedDevices = devices, devices
			switch mode {
			case "no-oci":
				source.Environment.PersistentResource = core.PersistentResourceRef{}
				delete(devices, "persistent-resource")
			case "missing-work":
				mounts = mounts[:1]
			case "missing-base":
				source.Environment.Base = nil
			case "extra-disk":
				devices["extra"] = map[string]string{"type": "disk", "path": "/extra", "source": "/foreign"}
			case "duplicate-work":
				mounts = append(mounts, mounts[0])
			case "foreign-owner":
				volumes[0].Config["user.hacocoon.owner"] = strings.Repeat("f", 32)
			case "running":
				instance.Status = "Running"
			case "foreign-instance":
				instance.Config[environmentInstanceKey] = "env-ffffffffffffffffffffffffffffffff"
			case "missing-volume":
				volumes = volumes[1:]
			case "foreign-user":
				volumes[0].UsedBy = []string{"/1.0/instances/haco-host?project=hacocoon"}
			case "readonly-option":
				devices[mounts[0].Device]["recursive"] = "true"
			case "wrong-pool":
				mounts[0].Pool = "foreign"
			}
			reads := 0
			r := New(&fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				if name != "incus" || len(args) != 2 || args[0] != "query" {
					t.Fatal("planner mutated provider", name, args)
				}
				reads++
				var value any
				switch {
				case strings.HasPrefix(args[1], "/1.0/instances?"):
					value = []snapshotInstanceObservation{instance}
				case args[1] == "/1.0/storage-pools/pool":
					value = map[string]string{"name": "pool", "driver": "btrfs"}
				case strings.Contains(args[1], "/volumes/custom?"):
					value = volumes
				case strings.HasPrefix(args[1], "/1.0/images/"):
					if mode == "missing-image" {
						return host.Result{ExitCode: 1}, nil
					}
					fingerprint := strings.Repeat("b", 64)
					if mode == "wrong-image" {
						fingerprint = strings.Repeat("c", 64)
					}
					value = map[string]string{"fingerprint": fingerprint, "type": "container"}
				default:
					t.Fatal(args)
				}
				data, _ := json.Marshal(value)
				return host.Result{Stdout: string(data)}, nil
			}})
			r.ConfigureManagedWorkspaces(func(_ context.Context, path string) ([]WorkspaceAttachment, error) {
				if path != "managed:work" {
					t.Fatal(path)
				}
				return mounts, nil
			})
			components, err := r.PlanSnapshot(context.Background(), source, "snap-"+strings.Repeat("e", 32))
			if mode != "ok" && mode != "no-oci" {
				if err == nil || len(components) != 0 {
					t.Fatal("incomplete aggregate accepted", err, components)
				}
				return
			}
			want := 5
			if mode == "no-oci" {
				want = 4
			}
			if err != nil || len(components) != want || reads == 0 {
				t.Fatal(err, len(components), reads)
			}
			owners, refs, roles := map[string]bool{}, map[string]bool{}, map[string]bool{}
			for _, c := range components {
				if owners[c.Owner] || refs[c.NativeRef] || roles[c.Role] {
					t.Fatal("duplicate saved identity")
				}
				owners[c.Owner], refs[c.NativeRef], roles[c.Role] = true, true, true
				b, err := r.decodeSnapshotComponent(c)
				if err != nil {
					t.Fatal(err)
				}
				if b.Volume != nil && (b.Volume.Device == "" || b.Volume.Path == "") {
					t.Fatal("lost mount layout")
				}
			}
			if !roles["rootfs"] || !roles["base"] || !roles["workspace:work-one"] || !roles["workspace:work-two"] || roles["oci"] != (mode == "ok") {
				t.Fatal(roles)
			}
		})
	}
}
