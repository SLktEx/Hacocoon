//go:build linux

package incus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
)

func workspaceMeasurementConsumerFixture() (snapshotInstanceObservation, string) {
	generation := core.NewEnvironmentInstanceID()
	config := func() map[string]string {
		return map[string]string{environmentInstanceKey: generation, managedEnvironmentMarkerKey: managedEnvironmentMarkerValue}
	}
	return snapshotInstanceObservation{
		Name: "haco-measurement", Type: "container", Config: config(), ExpandedConfig: config(),
		ExpandedDevices: map[string]map[string]string{
			"root": {"type": "disk", "pool": "owned-pool", "path": "/"},
			"work": {"type": "disk", "pool": "owned-pool", "source": "haco-work-one", "path": "/workspace/one"},
		},
	}, generation
}

func TestWorkspaceMeasurementConsumerRequiresExactOwnershipAndState(t *testing.T) {
	for _, state := range []struct{ status, wanted int }{{102, 0}, {103, 0}, {110, 110}, {103, 103}, {102, 102}} {
		instance, generation := workspaceMeasurementConsumerFixture()
		if err := validateWorkspaceMeasurementConsumer(instance, state.status, "owned-pool", "haco-work-one", "haco-measurement", generation, state.wanted); err != nil {
			t.Fatal("rejected exact consumer", err)
		}
	}
	for _, tc := range []struct {
		name   string
		change func(*snapshotInstanceObservation)
	}{
		{"foreign_name", func(i *snapshotInstanceObservation) { i.Name = "haco-other" }},
		{"wrong_type", func(i *snapshotInstanceObservation) { i.Type = "virtual-machine" }},
		{"ephemeral", func(i *snapshotInstanceObservation) { i.Ephemeral = true }},
		{"recreated_generation", func(i *snapshotInstanceObservation) {
			i.Config[environmentInstanceKey] = core.NewEnvironmentInstanceID()
		}},
		{"expanded_generation", func(i *snapshotInstanceObservation) {
			i.ExpandedConfig[environmentInstanceKey] = core.NewEnvironmentInstanceID()
		}},
		{"missing_kind", func(i *snapshotInstanceObservation) { delete(i.Config, managedEnvironmentMarkerKey) }},
		{"missing_expanded_config", func(i *snapshotInstanceObservation) { i.ExpandedConfig = nil }},
		{"foreign_root", func(i *snapshotInstanceObservation) { i.ExpandedDevices["root"]["pool"] = "other" }},
		{"root_source", func(i *snapshotInstanceObservation) { i.ExpandedDevices["root"]["source"] = "/host/path" }},
		{"foreign_workspace", func(i *snapshotInstanceObservation) { i.ExpandedDevices["work"]["source"] = "haco-work-other" }},
		{"workspace_path", func(i *snapshotInstanceObservation) { i.ExpandedDevices["work"]["path"] = "/elsewhere" }},
		{"workspace_pool", func(i *snapshotInstanceObservation) { i.ExpandedDevices["work"]["pool"] = "other" }},
		{"duplicate_workspace", func(i *snapshotInstanceObservation) { i.ExpandedDevices["duplicate"] = i.ExpandedDevices["work"] }},
		{"duplicate_root", func(i *snapshotInstanceObservation) { i.ExpandedDevices["duplicate"] = i.ExpandedDevices["root"] }},
		{"missing_devices", func(i *snapshotInstanceObservation) { i.ExpandedDevices = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			instance, generation := workspaceMeasurementConsumerFixture()
			tc.change(&instance)
			if validateWorkspaceMeasurementConsumer(instance, 103, "owned-pool", "haco-work-one", "haco-measurement", generation, 0) == nil {
				t.Fatal("accepted unsafe consumer")
			}
		})
	}
	for _, state := range []struct{ status, wanted int }{{110, 0}, {0, 0}, {107, 0}, {102, 110}, {103, 110}, {110, 103}} {
		instance, generation := workspaceMeasurementConsumerFixture()
		if validateWorkspaceMeasurementConsumer(instance, state.status, "owned-pool", "haco-work-one", "haco-measurement", generation, state.wanted) == nil {
			t.Fatal("accepted stale or unexpected consumer state", state)
		}
	}
}

func TestWorkspaceMeasurementSelectsOnlyOneReadyMember(t *testing.T) {
	member := gitrepo.Object{Kind: "work", ID: "owned", Repository: "one", State: "ready"}
	for _, work := range []gitrepo.Object{member, {Kind: "work", Members: []gitrepo.Object{member, {Kind: "work", ID: "other", Repository: "two", State: "ready"}}}} {
		if got, err := workspaceMeasurementMember(work); err != nil || got.ID != member.ID {
			t.Fatal("rejected exact member", err)
		}
	}
	for _, work := range []gitrepo.Object{{}, {Kind: "work", Members: []gitrepo.Object{member, member}}, {Kind: "work", ID: "wrong", Repository: "two", State: "ready"}, {Kind: "work", ID: "incomplete", Repository: "one", State: "creating"}} {
		if _, err := workspaceMeasurementMember(work); err == nil {
			t.Fatal("accepted ambiguous or incomplete Workspace")
		}
	}
}

func TestStorageMeasurementPayloadIsBoundedAndRejectsLinks(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if hashes, err := storageMeasurementHashes(root); err != nil || len(hashes) != 0 {
		t.Fatal("empty payload after unlink must remain observable", err)
	}
	file := filepath.Join(root, "base")
	if err := os.WriteFile(file, []byte(strings.Repeat("a", storageMeasurementFileBytes)), 0600); err != nil {
		t.Fatal(err)
	}
	if hashes, err := storageMeasurementHashes(root); err != nil || len(hashes) != 1 || len(hashes["base"]) != 64 {
		t.Fatal("exact bounded payload rejected", err)
	}
	for _, tc := range []struct {
		name  string
		alter func(string) error
	}{
		{"symlink", func(path string) error { return os.Symlink(file, filepath.Join(path, "base")) }},
		{"hardlink", func(path string) error { return os.Link(file, filepath.Join(path, "base")) }},
		{"directory", func(path string) error { return os.Mkdir(filepath.Join(path, "base"), 0700) }},
		{"short", func(path string) error { return os.WriteFile(filepath.Join(path, "base"), []byte("short"), 0600) }},
		{"extra_file", func(path string) error { return os.WriteFile(filepath.Join(path, "other"), nil, 0600) }},
		{"oversized", func(path string) error {
			return os.WriteFile(filepath.Join(path, "base"), make([]byte, storageMeasurementFileBytes+1), 0600)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := t.TempDir()
			if err := tc.alter(path); err != nil {
				t.Fatal(err)
			}
			if _, err := storageMeasurementHashes(path); err == nil {
				t.Fatal("accepted unsafe payload")
			}
		})
	}
}

func TestWorkspaceMeasurementRequiresExtentsNotOnlyMatchingBytes(t *testing.T) {
	hash := strings.Repeat("a", 64)
	sample := workspaceStorageSample{Hashes: map[string]string{"base": hash}, Payload: storageByteSample{LogicalBytes: storageMeasurementFileBytes, AllocatedBytes: storageMeasurementFileBytes, ExtentTotalBytes: storageMeasurementFileBytes, ExtentSetSharedBytes: storageMeasurementFileBytes}}
	if err := requireSharedWorkspacePayload(sample, hash); err != nil {
		t.Fatal("rejected measured sharing", err)
	}
	sample.Payload.ExtentSetSharedBytes = 0
	sample.Payload.ExtentExclusiveBytes = storageMeasurementFileBytes
	if requireSharedWorkspacePayload(sample, hash) == nil {
		t.Fatal("identical bytes incorrectly established extent sharing")
	}
}
