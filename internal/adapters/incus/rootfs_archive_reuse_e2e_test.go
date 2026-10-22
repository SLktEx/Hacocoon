//go:build linux

package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"time"

	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	environmenttransfer "github.com/SLktEx/Hacocoon/internal/env/transfer"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/workspace"
)

// Transfer the already-written ordinary Image Env. The earlier aggregate export
// predates these payloads and cannot supply this measurement's input bytes.
func measureSnapshotArchiveRoundTrip(m rootfsStorageObserver, binary string, source rootfsStorageArea, sourceWork gitrepo.Object, service *workspace.Service, repositories *gitrepo.RepositoryService, importer *environmenttransfer.Importer) {
	m.t.Helper()
	must := func(stage string, err error) {
		m.t.Helper()
		if err != nil {
			m.t.Fatal("archive measurement failed", stage)
		}
	}
	source.label, source.files = "archive-source", true
	m.guest(source, `test "$(stat --format=%u:%g "$1/base")" = 0:0; test "$(stat --format=%u:%g "$1/delta")" = 0:0`)
	must("stop", service.StopForWorkspace(m.ctx, source.live.env.Name, source.live.env.Workspace.ID))
	m.live(*source.live, 102)
	before := m.observe("archive_before_export", source)[source.label]
	snapshots, err := service.ListSnapshots(m.ctx, "")
	must("snapshot_inventory", err)
	requireArchiveTemporaryCleanup(m, importer.Root)
	archivePath := filepath.Join(importer.Root, "rootfs-materialization.haco")
	output, err := aggregateCLIOutput(m.ctx, binary, "env", "export", "--json", source.live.env.Name, archivePath)
	must("public_export", err)
	var exported struct {
		File   string                             `json:"file"`
		Result controlapi.EnvironmentExportResult `json:"result"`
	}
	if json.Unmarshal(output, &exported) != nil || exported.File != archivePath || exported.Result.TemporarySnapshot != "" {
		m.t.Fatal("archive export receipt invalid")
	}
	inspectCtx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
	archive, manifest, retained, err := openArchiveStorageMeasurement(inspectCtx, archivePath, 4<<30)
	cancel()
	must("retained_archive", err)
	defer func() { must("archive_close", archive.Close()) }()
	if exported.Result.Bytes != int64(retained.LogicalBytes) || exported.Result.SHA256 != retained.SHA256 || manifest.Source != source.live.env.Name || manifest.Version != 2 || manifest.HasOCI != (source.live.env.PersistentResource.ID != "") || len(manifest.Workspaces) != len(sourceWork.Copies()) || len(manifest.Data) != 0 {
		m.t.Fatal("retained archive differs from source inventory or export receipt")
	}
	logArchiveStorageSample(m, "archive_after_export", retained)
	currentSnapshots, err := service.ListSnapshots(m.ctx, "")
	must("snapshot_cleanup_inventory", err)
	if !reflect.DeepEqual(snapshots, currentSnapshots) {
		m.t.Fatal("archive temporary Snapshot cleanup unproven")
	}
	requireArchiveTemporaryCleanup(m, importer.Root)
	m.live(*source.live, 102)
	afterExport := m.observe("archive_after_export", source)[source.label]
	must("source_after_export", requireArchivePayloadUnchanged(before, afterExport))

	name := source.live.env.Name + "-imported"
	output, err = aggregateCLIOutput(m.ctx, binary, "env", "import", "--json", archivePath, name)
	must("public_import", err)
	var receipt environmenttransfer.ImportResult
	if json.Unmarshal(output, &receipt) != nil || receipt.Environment != name || receipt.State != "running" || receipt.Workspace == sourceWork.ID || (receipt.OCI != "") != manifest.HasOCI {
		m.t.Fatal("archive import receipt invalid")
	}
	imported, err := m.catalog.GetEnvironment(m.ctx, name)
	must("imported_environment", err)
	generation, err := m.catalog.EnvironmentInstance(m.ctx, imported)
	must("imported_generation", err)
	work, err := repositories.Get("work", receipt.Workspace)
	must("imported_workspace", err)
	if imported.Base != nil || imported.OwnedWorkspace || imported.Workspace.ID == source.live.env.Workspace.ID || imported.Workspace.Path != "managed:"+work.ID || generation == source.live.generation || work.Owner == sourceWork.Owner || work.State != "ready" {
		m.t.Fatal("archive import fresh ownership unproven")
	}
	lease, err := m.catalog.GetWorkspaceLease(m.ctx, name)
	must("imported_lease", err)
	if lease.State != core.WorkspaceLeaseActive || lease.InstanceID != generation || lease.SnapshotSource != "" || lease.WorkspaceID != imported.Workspace.ID {
		m.t.Fatal("archive import lifecycle publication unproven")
	}
	var store core.PersistentResource
	if receipt.OCI != "" {
		store, err = m.catalog.GetPersistentResource(m.ctx, receipt.OCI)
		must("imported_store", err)
		if store.Ref() != imported.PersistentResource || store.WorkspaceID != imported.Workspace.ID || store.ID == source.live.env.PersistentResource.ID || store.Owner == source.live.env.PersistentResource.Owner || store.State != "ready" {
			m.t.Fatal("archive import Store ownership unproven")
		}
	}
	for _, member := range work.Copies() {
		for _, original := range sourceWork.Copies() {
			if member.Owner == original.Owner || member.NativeRef == original.NativeRef {
				m.t.Fatal("archive import reused source Workspace volume")
			}
		}
		must("imported_volume_ownership", (&RepositoryBackend{Runtime: m.runtime}).InspectVolume(m.ctx, member))
	}
	destination := rootfsStorageArea{label: "archive-imported", live: &rootfsLiveIdentity{env: imported, generation: generation}, files: true}
	m.guest(destination, `test "$(stat --format=%u:%g "$1/base")" = 0:0; test "$(stat --format=%u:%g "$1/delta")" = 0:0`)
	m.live(*source.live, 102)
	after := m.observe("archive_after_import", source, destination)
	must("source_after_import", requireArchivePayloadUnchanged(before, after[source.label]))
	must("imported_payload", requireArchivePayloadUnchanged(before, after[destination.label]))
	relationship, err := archivePayloadMaterialization(before, after[source.label], after[destination.label])
	must("materialization_observation", err)
	m.t.Logf("storage_measurement fixture=%s phase=archive_after_import payload_role=delta relationship=%s", rootfsMeasurementFixture, relationship)
	observeArchive := func(phase string) {
		ctx, cancel := context.WithTimeout(m.ctx, 2*time.Minute)
		defer cancel()
		sample, err := archive.Observe(ctx)
		must("retained_archive_recheck", err)
		logArchiveStorageSample(m, phase, sample)
	}
	observeArchive("archive_after_import")
	requireArchiveTemporaryCleanup(m, importer.Root)

	// Import leaves explicitly retained data. Delete through canonical lifecycle,
	// then exact-owned cleanup; never delete by a guessed native name or path.
	must("destination_delete", service.Delete(m.ctx, name))
	must("destination_data_cleanup", service.CleanupRestoredData(m.ctx, imported.Workspace, func(ctx context.Context) error {
		if store.ID != "" {
			if err := importer.Stores.DeleteRestoredCopy(ctx, store); err != nil {
				return err
			}
		}
		return repositories.DeleteWorkspace(ctx, work.ID, work.Owner)
	}))
	if exists, err := m.runtime.environmentExists(m.ctx, "haco-"+name); err != nil || exists {
		m.t.Fatal("archive destination provider absence unproven")
	}
	if _, err := m.catalog.GetEnvironment(m.ctx, name); !errors.Is(err, core.ErrNotFound) {
		m.t.Fatal("archive destination catalog absence unproven")
	}
	if _, err := m.catalog.GetWorkspaceLease(m.ctx, name); !errors.Is(err, core.ErrNotFound) {
		m.t.Fatal("archive destination lease absence unproven")
	}
	if _, err := m.catalog.OwnedWorkspaceCleanup(m.ctx, name); !errors.Is(err, core.ErrNotFound) {
		m.t.Fatal("archive destination data cleanup remains pending")
	}
	if _, err := repositories.Get("work", work.ID); !errors.Is(err, core.ErrNotFound) {
		m.t.Fatal("archive destination Workspace retained")
	}
	for _, member := range work.Copies() {
		pool, volume, err := volumeRef(member)
		must("destination_volume_identity", err)
		var inventory []repositoryVolumeObservation
		if json.Unmarshal([]byte(m.command("incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+m.runtime.project+"&recursion=1")), &inventory) != nil || inventory == nil {
			m.t.Fatal("archive destination volume absence unavailable")
		}
		for _, item := range inventory {
			if item.Name == volume {
				m.t.Fatal("archive destination Workspace volume retained")
			}
		}
	}
	if store.ID != "" {
		if _, err := m.catalog.GetPersistentResource(m.ctx, store.ID); !errors.Is(err, core.ErrNotFound) {
			m.t.Fatal("archive destination Store retained")
		}
		if observed, err := (&PersistentResourceBackend{Runtime: m.runtime}).observe(m.ctx, store); err != nil || observed != nil {
			m.t.Fatal("archive destination Store absence unproven")
		}
	}
	m.live(*source.live, 102)
	final := m.observe("archive_destination_deleted", source)[source.label]
	must("source_after_cleanup", requireArchivePayloadUnchanged(before, final))
	observeArchive("archive_destination_deleted")
	requireArchiveTemporaryCleanup(m, importer.Root)
	m.t.Log("PASS retained native archive round trip: shipped CLI/controller export/import, manifest-role bytes and digests, fresh owned destination, unchanged source payload and retained archive, exact temporary/destination cleanup; no transient peak or device-write measurement")
}

func requireArchiveTemporaryCleanup(m rootfsStorageObserver, root string) {
	m.t.Helper()
	for _, pattern := range []string{"rootfs-export-*.jsonl", "rootfs-import-*.jsonl"} {
		pending, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil || len(pending) != 0 {
			m.t.Fatal("archive temporary image cleanup unproven")
		}
	}
}

func logArchiveStorageSample(m rootfsStorageObserver, phase string, sample archiveStorageSample) {
	m.t.Helper()
	encoded, err := json.Marshal(sample)
	if err != nil {
		m.t.Fatal("archive measurement encoding failed")
	}
	m.t.Logf("storage_measurement fixture=%s phase=%s area=retained-archive bytes=%s", rootfsMeasurementFixture, phase, encoded)
}

func requireArchivePayloadUnchanged(before, after rootfsStorageSample) error {
	if len(before.Hashes) != 2 || len(after.Hashes) != 2 || !reflect.DeepEqual(before.Hashes, after.Hashes) || len(before.Files) != 2 || len(after.Files) != 2 {
		return fmt.Errorf("archive payload inventory or content changed")
	}
	for _, role := range []string{"base", "delta"} {
		if !baseFingerprintPattern.MatchString(before.Hashes[role]) {
			return fmt.Errorf("archive payload digest unavailable")
		}
		for _, sample := range []rootfsStorageSample{before, after} {
			file, ok := sample.Files[role]
			if !ok || file.LogicalBytes != storageMeasurementFileBytes || file.AllocatedBytes == 0 || file.ExtentTotalBytes == 0 {
				return fmt.Errorf("archive payload file accounting unavailable")
			}
		}
	}
	return nil
}

// Nonzero set-shared in separate calls cannot identify which files share. A
// wholly exclusive source delta excludes sharing with any retained destination;
// destination sharing with its own cache does not weaken that bounded finding.
func archivePayloadMaterialization(before, source, imported rootfsStorageSample) (string, error) {
	if err := requireArchivePayloadUnchanged(before, source); err != nil {
		return "", err
	}
	if err := requireArchivePayloadUnchanged(before, imported); err != nil {
		return "", err
	}
	exclusive := func(s storageByteSample) bool {
		return s.ExtentTotalBytes > 0 && s.ExtentExclusiveBytes == s.ExtentTotalBytes && s.ExtentSetSharedBytes == 0
	}
	if !exclusive(before.Files["delta"]) {
		return "inconclusive_baseline_shared", nil
	}
	if !exclusive(source.Files["delta"]) {
		return "inconclusive_source_shared", nil
	}
	return "separately_materialized_from_source", nil
}
