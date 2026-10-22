//go:build linux

package incus

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/env"
)

func rootfsMeasurementConsumerFixture() (snapshotInstanceObservation, string) {
	i, generation := workspaceMeasurementConsumerFixture()
	delete(i.ExpandedDevices, "work")
	i.Devices = map[string]map[string]string{"root": {"type": "disk", "pool": "owned-pool", "path": "/"}}
	i.Config["volatile.base_image"] = strings.Repeat("a", 64)
	i.ExpandedConfig["volatile.base_image"] = strings.Repeat("a", 64)
	return i, generation
}

func TestRootfsMeasurementConsumerRequiresExactOwnershipAndImage(t *testing.T) {
	fp := strings.Repeat("a", 64)
	for _, state := range []struct{ status, wanted int }{{102, 0}, {103, 0}, {110, 110}, {103, 103}, {102, 102}} {
		i, id := rootfsMeasurementConsumerFixture()
		if err := validateRootfsMeasurementConsumer(i, state.status, "owned-pool", "haco-measurement", id, fp, state.wanted); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*snapshotInstanceObservation)
	}{
		{"foreign_name", func(i *snapshotInstanceObservation) { i.Name = "haco-other" }},
		{"wrong_type", func(i *snapshotInstanceObservation) { i.Type = "virtual-machine" }},
		{"ephemeral", func(i *snapshotInstanceObservation) { i.Ephemeral = true }},
		{"generation", func(i *snapshotInstanceObservation) {
			i.Config[environmentInstanceKey] = core.NewEnvironmentInstanceID()
		}},
		{"expanded_generation", func(i *snapshotInstanceObservation) { i.ExpandedConfig[environmentInstanceKey] = "" }},
		{"missing_marker", func(i *snapshotInstanceObservation) { delete(i.Config, managedEnvironmentMarkerKey) }},
		{"wrong_image", func(i *snapshotInstanceObservation) { i.Config["volatile.base_image"] = strings.Repeat("b", 64) }},
		{"missing_expanded_image", func(i *snapshotInstanceObservation) { delete(i.ExpandedConfig, "volatile.base_image") }},
		{"foreign_pool", func(i *snapshotInstanceObservation) { i.ExpandedDevices["root"]["pool"] = "other" }},
		{"root_source", func(i *snapshotInstanceObservation) { i.ExpandedDevices["root"]["source"] = "/host" }},
		{"duplicate_root", func(i *snapshotInstanceObservation) { i.ExpandedDevices["duplicate"] = i.ExpandedDevices["root"] }},
		{"payload_mount", func(i *snapshotInstanceObservation) {
			i.ExpandedDevices["payload"] = map[string]string{"type": "disk", "path": rootfsMeasurementDirectory, "source": "foreign"}
		}},
		{"payload_descendant_mount", func(i *snapshotInstanceObservation) {
			i.ExpandedDevices["payload"] = map[string]string{"type": "disk", "path": rootfsMeasurementDirectory + "/base", "source": "foreign"}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i, id := rootfsMeasurementConsumerFixture()
			tc.change(&i)
			if validateRootfsMeasurementConsumer(i, 103, "owned-pool", "haco-measurement", id, fp, 0) == nil {
				t.Fatal("accepted unsafe rootfs consumer")
			}
		})
	}
	for _, state := range []struct{ status, wanted int }{{110, 0}, {0, 0}, {107, 0}, {102, 110}, {103, 110}, {110, 103}} {
		i, id := rootfsMeasurementConsumerFixture()
		if validateRootfsMeasurementConsumer(i, state.status, "owned-pool", "haco-measurement", id, fp, state.wanted) == nil {
			t.Fatal("accepted changed state")
		}
	}
}

func TestRootfsMeasurementCacheRequiresExactImmutableVolume(t *testing.T) {
	fp := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, kind, content, readonly string
		valid                         bool
	}{
		{fp, "image", "filesystem", "ro=true\n", true},
		{strings.Repeat("b", 64), "image", "filesystem", "ro=true", false},
		{fp, "custom", "filesystem", "ro=true", false},
		{fp, "image", "block", "ro=true", false},
		{fp, "image", "filesystem", "ro=false", false},
		{fp, "image", "filesystem", "ro=true\nextra", false},
		{"", "image", "filesystem", "ro=true", false},
	} {
		if err := validateRootfsMeasurementImageVolume(tc.name, tc.kind, tc.content, tc.readonly, fp); (err == nil) != tc.valid {
			t.Fatalf("unexpected cache validation: %v", err)
		}
	}
}

func TestRootfsMeasurementSharingNeedsExtentsAndUnchangedPeers(t *testing.T) {
	hash := strings.Repeat("a", 64)
	before := rootfsStorageSample{Hashes: map[string]string{"base": hash}, Payload: storageByteSample{LogicalBytes: 8 << 20, AllocatedBytes: 8 << 20, ExtentTotalBytes: 8 << 20, ExtentSetSharedBytes: 8 << 20}}
	if err := requireSharedRootfsPayload(before, hash); err != nil {
		t.Fatal(err)
	}
	noSharing := before
	noSharing.Payload.ExtentSetSharedBytes = 0
	noSharing.Payload.ExtentExclusiveBytes = 8 << 20
	if requireSharedRootfsPayload(noSharing, hash) == nil {
		t.Fatal("hash equality must not imply sharing")
	}
	after := before
	after.Hashes = map[string]string{"base": hash, "delta": strings.Repeat("b", 64)}
	after.Payload.LogicalBytes += 8 << 20
	after.Payload.AllocatedBytes += 8 << 20
	after.Payload.ExtentExclusiveBytes += 8 << 20
	after.Payload.ExtentTotalBytes += 8 << 20
	if err := requireIndependentRootfsWrite(before, after, hash); err != nil {
		t.Fatal(err)
	}
	after.Hashes["base"] = "changed"
	if requireIndependentRootfsWrite(before, after, hash) == nil {
		t.Fatal("accepted changed original payload")
	}
}

func rootfsMeasurementSavedFixture(t *testing.T) (*Runtime, core.Snapshot) {
	t.Helper()
	r := &Runtime{project: "hacocoon"}
	root := snapshotRootfsPlan{Pool: "owned-pool", Source: "haco-measurement", SourceInstanceID: core.NewEnvironmentInstanceID(), Owner: strings.Repeat("a", 32)}
	image := snapshotImagePlan{Name: "saved-measurement", Owner: strings.Repeat("b", 32), Rootfs: root}
	saved := core.Snapshot{ID: image.Name, State: "ready", Source: core.SnapshotSource{Environment: core.Environment{Name: "measurement", RuntimeRef: root.Source, Workspace: core.Workspace{ID: "owned-workspace"}}, InstanceID: root.SourceInstanceID}, Image: &core.BaseRef{Name: core.BaseName(image.Name), Revision: core.BaseRevision("sha256:" + strings.Repeat("c", 64))}}
	for _, binding := range []snapshotBinding{{Version: 1, Project: r.project, Rootfs: &root}, {Version: 1, Project: r.project, Image: &image}} {
		component, err := r.snapshotComponent(binding)
		if err != nil {
			t.Fatal(err)
		}
		component.State = "verified"
		component.NativeRef = "haco-runtime-v1:" + environmentapp.ProviderIncus + ":" + base64.RawURLEncoding.EncodeToString([]byte(component.NativeRef))
		saved.Components = append(saved.Components, component)
	}
	return r, saved
}

func TestRootfsMeasurementSavedPlansKeepExactRootAndImageIdentity(t *testing.T) {
	r, saved := rootfsMeasurementSavedFixture(t)
	root, image, err := rootfsMeasurementPlans(r, saved)
	if err != nil || root != image.Rootfs {
		t.Fatal("exact saved identities rejected", err)
	}
	for _, tc := range []struct {
		name   string
		change func(*core.Snapshot)
	}{
		{"missing_root", func(s *core.Snapshot) { s.Components = s.Components[1:] }},
		{"missing_image", func(s *core.Snapshot) { s.Components = s.Components[:1] }},
		{"duplicate_root", func(s *core.Snapshot) { s.Components = append(s.Components, s.Components[0]) }},
		{"duplicate_image", func(s *core.Snapshot) { s.Components = append(s.Components, s.Components[1]) }},
		{"unverified", func(s *core.Snapshot) { s.Components[0].State = "created" }},
		{"foreign_route", func(s *core.Snapshot) { s.Components[0].NativeRef = "haco-runtime-v1:other:abc" }},
		{"foreign_owner", func(s *core.Snapshot) { s.Components[0].Owner = strings.Repeat("d", 32) }},
		{"wrong_source_generation", func(s *core.Snapshot) { s.Source.InstanceID = core.NewEnvironmentInstanceID() }},
		{"wrong_source_runtime", func(s *core.Snapshot) { s.Source.Environment.RuntimeRef = "haco-other" }},
		{"wrong_image_name", func(s *core.Snapshot) { s.Image.Name = "other" }},
		{"wrong_image_revision", func(s *core.Snapshot) { s.Image.Revision = "latest" }},
		{"missing_image_ref", func(s *core.Snapshot) { s.Image = nil }},
		{"different_image_root", func(s *core.Snapshot) {
			changed := image
			changed.Rootfs.Owner = strings.Repeat("d", 32)
			component, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: r.project, Image: &changed})
			if err != nil {
				t.Fatal(err)
			}
			component.State = "verified"
			component.NativeRef = "haco-runtime-v1:" + environmentapp.ProviderIncus + ":" + base64.RawURLEncoding.EncodeToString([]byte(component.NativeRef))
			s.Components[1] = component
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, saved := rootfsMeasurementSavedFixture(t)
			tc.change(&saved)
			if _, _, err := rootfsMeasurementPlans(r, saved); err == nil {
				t.Fatal("accepted ambiguous saved identity")
			}
		})
	}
}

func TestRootfsMeasurementAreaRejectsAmbiguousKinds(t *testing.T) {
	_, saved := rootfsMeasurementSavedFixture(t)
	live := &rootfsLiveIdentity{env: saved.Source.Environment, generation: saved.Source.InstanceID}
	for _, area := range []rootfsStorageArea{{label: "live", live: live}, {label: "saved", saved: &saved}, {label: "image", image: &saved}} {
		if _, err := rootfsMeasurementSource(area); err != nil {
			t.Fatal("rejected typed identity", err)
		}
	}
	for _, area := range []rootfsStorageArea{{label: "missing"}, {label: "both", live: live, saved: &saved}, {label: "both", saved: &saved, image: &saved}, {live: live}, {label: "empty", live: &rootfsLiveIdentity{}}} {
		if _, err := rootfsMeasurementSource(area); err == nil {
			t.Fatal("accepted missing or ambiguous identity")
		}
	}
}

func TestRootfsMeasurementRejectsOverlappingHostMounts(t *testing.T) {
	const pool = "/var/lib/incus/storage-pools/owned-pool"
	const root = pool + "/containers/project_env/rootfs"
	for _, tc := range []struct {
		mount string
		valid bool
	}{
		{"/", true}, {pool, true}, {pool + "/containers/other/rootfs/proc", true}, {pool + "/containers/project_env/rootfs-peer", true},
		{pool + "/containers", false}, {pool + "/containers/project_env", false}, {root, false}, {root + "/proc", false}, {root + "/workspace/one", false},
	} {
		raw := `{"filesystems":[{"target":"` + pool + `"},{"target":"` + tc.mount + `"}]}`
		if tc.mount == pool {
			raw = `{"filesystems":[{"target":"` + pool + `"}]}`
		}
		if err := validateRootfsMeasurementMountScope(raw, pool, []string{root}); (err == nil) != tc.valid {
			t.Fatalf("unexpected mount-scope result for %s: %v", tc.mount, err)
		}
	}
	for _, raw := range []string{`{"filesystems":[{"target":"` + pool + `"},{"target":"` + pool + `"}]}`, `{}`, `null`, `{"filesystems":null}`, `{"filesystems":[]}`, `{"filesystems":[{"target":"relative"}]}`, `{"filesystems":[{"target":"/"}]}`, `{"filesystems":[{"target":"/var/../var"}]}`, `{"filesystems":[{"target":"/","children":[{"target":"` + root + `"}]}]}`} {
		if validateRootfsMeasurementMountScope(raw, pool, []string{root}) == nil {
			t.Fatal("accepted missing or malformed mount inventory")
		}
	}
	// JSON escapes are decoded before path comparisons. Escaping must not hide
	// an exact root or descendant; unrelated escaped names remain permitted.
	for _, target := range []string{strings.ReplaceAll(root, "/", `\u002f`), root + `/work\u0073pace`} {
		raw := `{"filesystems":[{"target":"` + pool + `"},{"target":"` + target + `"}]}`
		if validateRootfsMeasurementMountScope(raw, pool, []string{root}) == nil {
			t.Fatal("escaped mount hid overlap")
		}
	}
	for _, raw := range []string{`{"filesystems":[{"target":"` + pool + `"}`, strings.Repeat(" ", (2<<20)+1)} {
		if validateRootfsMeasurementMountScope(raw, pool, []string{root}) == nil {
			t.Fatal("accepted truncated or overlong mount inventory")
		}
	}
	if validateRootfsMeasurementMountScope(`{"filesystems":[{"target":"`+pool+`"}]}`, pool, []string{"/outside"}) == nil {
		t.Fatal("accepted rootfs outside pool")
	}
}
