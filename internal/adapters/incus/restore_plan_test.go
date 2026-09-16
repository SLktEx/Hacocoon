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

func restorePlanFixture(t *testing.T, mode string) (*Runtime, core.Snapshot) {
	t.Helper()
	root, _ := rootfsFixture()
	volume := snapshotVolumeFixture("work")
	volume.SourceInstance = root.Source
	volume.SourceInstanceID = root.SourceInstanceID
	rootConfig := root.config()
	volumeConfig := volume.targetConfig()
	devices := map[string]map[string]string{"root": {"type": "disk", "path": "/", "pool": "pool"}}
	instance := snapshotInstanceObservation{Name: root.target(), Type: "container", Status: "Stopped", Config: rootConfig, ExpandedConfig: rootConfig, Devices: devices, ExpandedDevices: devices}
	data := persistentVolumeObservation{Name: volume.target(), Type: "custom", ContentType: "filesystem", Config: volumeConfig}
	switch mode {
	case "foreign rootfs":
		rootConfig["user.hacocoon.owner"] = strings.Repeat("f", 32)
	case "busy volume":
		data.UsedBy = []string{"/1.0/instances/other?project=hacocoon"}
	}
	r := New(&fakeRunner{run: func(_ context.Context, _ int, command string, args []string) (host.Result, error) {
		if command != "incus" || len(args) != 2 || args[0] != "query" {
			t.Fatalf("planning mutated provider data: %s %q", command, args)
		}
		if mode == "unavailable" {
			return host.Result{ExitCode: 1, Stdout: "[]"}, nil
		}
		var observation any
		switch args[1] {
		case "/1.0/instances?project=hacocoon&recursion=1":
			observation = []snapshotInstanceObservation{instance}
		case "/1.0/storage-pools/pool/volumes/custom?project=hacocoon&recursion=1":
			observation = []persistentVolumeObservation{data}
			if mode == "missing volume" {
				observation = []persistentVolumeObservation{}
			}
		default:
			t.Fatalf("planning consulted unrelated provider state: %q", args)
		}
		encoded, err := json.Marshal(observation)
		return host.Result{Stdout: string(encoded)}, err
	}})
	saved := core.Snapshot{State: "ready"}
	// Legacy Base ownership remains in the source manifest. Its native instance
	// is deliberately absent: complete rootfs restoration must not require it.
	base := baseSnapshotFixture()
	for _, binding := range []snapshotBinding{{Version: 1, Project: "hacocoon", Base: &base}, {Version: 1, Project: "hacocoon", Rootfs: &root}, {Version: 1, Project: "hacocoon", Volume: &volume}} {
		component, err := r.snapshotComponent(binding)
		if err != nil {
			t.Fatal(err)
		}
		component.State = "verified"
		saved.Components = append(saved.Components, component)
	}
	return r, saved
}

func TestRestorePlanKeepsSavedOwnershipAndAllocatesIndependentTargets(t *testing.T) {
	r, saved := restorePlanFixture(t, "")
	before, err := json.Marshal(saved)
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]bool{}
	for _, c := range saved.Components {
		owners[c.Owner] = true
	}
	for attempt := 0; attempt < 2; attempt++ {
		plan, err := r.PlanSnapshotRestore(context.Background(), saved, "restore-"+strings.Repeat("d", 32))
		if err != nil || len(plan) != 2 {
			t.Fatalf("complete rootfs/Workspace plan = %+v, %v", plan, err)
		}
		for i, c := range plan {
			if c.State != "planned" || c.Role != saved.Components[i+1].Role || owners[c.Owner] || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:restore", Owner: c.Owner}) {
				t.Fatalf("restore inherited source or previous attempt ownership: %+v", c)
			}
			owners[c.Owner] = true
			prefix := "volume/pool/"
			if c.Role == "rootfs" {
				prefix = "instance/"
			}
			if c.NativeRef != prefix+"haco-restore-"+c.Owner {
				t.Fatalf("restore destination does not bind fresh owner: %+v", c)
			}
			var receipt struct {
				Owner  string
				Source core.SnapshotComponent
			}
			if err := json.Unmarshal([]byte(c.Binding), &receipt); err != nil || receipt.Owner != c.Owner || receipt.Source != saved.Components[i+1] {
				t.Fatal("restore receipt lost the exact immutable source", err)
			}
		}
	}
	after, err := json.Marshal(saved)
	if err != nil || string(after) != string(before) {
		t.Fatal("planning rewrote saved components or legacy Base ownership")
	}
}

func TestRestorePlanRefusesIncompleteOrUnverifiableSourcesWithoutPartialPlan(t *testing.T) {
	for _, mode := range []string{"foreign rootfs", "busy volume", "missing volume", "unavailable", "invalid id", "not ready", "short manifest", "oversized manifest", "unverified component", "binding drift"} {
		t.Run(mode, func(t *testing.T) {
			r, saved := restorePlanFixture(t, mode)
			id := "restore-" + strings.Repeat("d", 32)
			want := core.ErrInvalidArgument
			switch mode {
			case "foreign rootfs", "binding drift":
				want = core.ErrCapabilityStale
			case "busy volume":
				want = core.ErrStorageBusy
			case "missing volume":
				want = core.ErrNotFound
			case "unavailable":
				want = core.ErrRuntimeUnavailable
			case "invalid id":
				id = "restore-" + strings.Repeat("z", 32)
			case "not ready":
				saved.State = "creating"
			case "short manifest":
				saved.Components = saved.Components[1:2]
			case "oversized manifest":
				saved.Components = make([]core.SnapshotComponent, 257)
			case "unverified component":
				saved.Components[2].State = "planned"
				want = core.ErrIncompatibleState
			}
			if mode == "binding drift" {
				saved.Components[2].Owner = strings.Repeat("e", 32)
			}
			before := append([]core.SnapshotComponent(nil), saved.Components...)
			plan, err := r.PlanSnapshotRestore(context.Background(), saved, id)
			if !errors.Is(err, want) || plan != nil || !reflect.DeepEqual(before, saved.Components) {
				t.Fatalf("failed planning returned partial destinations or changed source: plan=%+v err=%v, want %v", plan, err, want)
			}
		})
	}
}
