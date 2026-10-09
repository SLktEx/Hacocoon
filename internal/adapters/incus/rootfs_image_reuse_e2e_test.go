//go:build linux

package incus

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmenttransfer "github.com/SLktEx/Hacocoon/internal/env/transfer"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/workspace"
)

// Reuse the aggregate fixture's existing creation controller and ordinary Image
// path. No new build, pool, permission or workspace source is provisioned here.
func measureSnapshotImageReuse(m rootfsStorageObserver, binary string, saved core.Snapshot, hash string, service *workspace.Service, repositories *gitrepo.RepositoryService, importer *environmenttransfer.Importer) {
	m.t.Helper()
	must := func(err error) {
		m.t.Helper()
		if err != nil {
			m.t.Fatal(err)
		}
	}
	if saved.Image == nil {
		m.t.Fatal("Snapshot-generated Image missing")
	}
	fingerprint, err := baseRevisionFingerprint(saved.Image.Revision)
	must(err)
	var areas []rootfsStorageArea
	var works []gitrepo.Object
	for _, suffix := range []string{"image-a", "image-b"} {
		name := saved.Source.Environment.Name + "-" + suffix
		output, err := aggregateCLIOutput(m.ctx, binary, "open", "--new", string(saved.Image.Name), "--name", name, "--client", "none", "--json")
		if err != nil {
			m.t.Fatal("ordinary Snapshot-generated Image creation failed", err)
		}
		var returned core.Environment
		if json.Unmarshal(output, &returned) != nil || returned.Name != name {
			m.t.Fatal("ordinary Image creation receipt invalid")
		}
		current, err := m.catalog.GetEnvironment(m.ctx, name)
		must(err)
		generation, err := m.catalog.EnvironmentInstance(m.ctx, current)
		must(err)
		if current.Base == nil || *current.Base != *saved.Image || !current.OwnedWorkspace || generation == saved.Source.InstanceID {
			m.t.Fatal("ordinary Image provenance or fresh ownership unproven")
		}
		lease, err := m.catalog.GetWorkspaceLease(m.ctx, name)
		must(err)
		if lease.SnapshotSource != "" || lease.State != core.WorkspaceLeaseActive {
			m.t.Fatal("ordinary Image creation used saved-rootfs route")
		}
		work, err := repositories.Get("work", strings.TrimPrefix(current.Workspace.Path, "managed:"))
		must(err)
		works = append(works, work)
		areas = append(areas, rootfsStorageArea{label: suffix, live: &rootfsLiveIdentity{env: current, generation: generation, fingerprint: fingerprint}})
	}
	if areas[0].live.generation == areas[1].live.generation || areas[0].live.env.Workspace.ID == areas[1].live.env.Workspace.ID {
		m.t.Fatal("ordinary Image Envs share authority or writable Workspace")
	}
	cache := rootfsStorageArea{label: "image-cache", image: &saved}
	savedArea := rootfsStorageArea{label: "saved", saved: &saved}
	// Cache creation is lazy during the first ordinary init. Publication itself
	// can export/materialize new extents; never assert saved-to-cache sharing.
	before := m.observe("after_two_image_envs", cache, areas[0], areas[1], savedArea)
	for _, area := range []string{"image-cache", "image-a", "image-b"} {
		must(requireSharedRootfsPayload(before[area], hash))
	}
	if len(before["saved"].Hashes) != 1 || before["saved"].Hashes["base"] != hash {
		m.t.Fatal("Image reuse changed saved rootfs")
	}
	m.guest(areas[1], `test ! -e "$1/delta"; dd if=/dev/urandom of="$1/delta" bs=1048576 count=8 status=none conv=fsync`)
	after := m.observe("after_image_env_write", cache, areas[0], areas[1], savedArea)
	must(requireIndependentRootfsWrite(before["image-b"], after["image-b"], hash))
	for _, area := range []string{"image-cache", "image-a"} {
		must(requireSharedRootfsPayload(after[area], hash))
	}
	if len(after["saved"].Hashes) != 1 || after["saved"].Hashes["base"] != hash {
		m.t.Fatal("ordinary Image Env write changed saved rootfs")
	}
	measureSnapshotArchiveRoundTrip(m, binary, areas[1], works[1], service, repositories, importer)
	for i, area := range areas {
		// The canonical lifecycle owns stop, runtime absence and automatic data
		// cleanup. On failure retain the durable receipts; no raw cache deletion.
		must(service.Delete(m.ctx, area.live.env.Name))
		if exists, err := m.runtime.environmentExists(m.ctx, "haco-"+area.live.env.Name); err != nil || exists {
			m.t.Fatal("Image Env provider absence unproven")
		}
		if _, err := m.catalog.GetEnvironment(m.ctx, area.live.env.Name); !errors.Is(err, core.ErrNotFound) {
			m.t.Fatal("Image Env catalog cleanup unproven")
		}
		if _, err := m.catalog.GetWorkspaceLease(m.ctx, area.live.env.Name); !errors.Is(err, core.ErrNotFound) {
			m.t.Fatal("Image Env lease cleanup unproven")
		}
		if _, err := m.catalog.OwnedWorkspaceCleanup(m.ctx, area.live.env.Name); !errors.Is(err, core.ErrNotFound) {
			m.t.Fatal("Image Env data cleanup remains pending")
		}
		if _, err := repositories.Get("work", works[i].ID); !errors.Is(err, core.ErrNotFound) {
			m.t.Fatal("Image Env Workspace retained")
		}
		for _, member := range works[i].Copies() {
			pool, volume, err := volumeRef(member)
			must(err)
			var inventory []repositoryVolumeObservation
			if json.Unmarshal([]byte(m.command("incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+m.runtime.project+"&recursion=1")), &inventory) != nil || inventory == nil {
				m.t.Fatal("Image Env Workspace absence unavailable")
			}
			for _, item := range inventory {
				if item.Name == volume {
					m.t.Fatal("Image Env Workspace volume retained")
				}
			}
		}
	}
	retained := m.observe("image_envs_deleted_cache_retained", cache, savedArea)
	if len(retained["image-cache"].Hashes) != 1 || retained["image-cache"].Hashes["base"] != hash || len(retained["saved"].Hashes) != 1 || retained["saved"].Hashes["base"] != hash {
		m.t.Fatal("Image Env deletion changed retained source content")
	}
	m.t.Log("PASS two ordinary Envs from one Snapshot-generated Image: pinned immutable cache, shared payload extents, independent guest write, unchanged cache/peer/saved rootfs, exact-owned Env and automatic Workspace cleanup; general Base builds and publication sharing not measured")
}
