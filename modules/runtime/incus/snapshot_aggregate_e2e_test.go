//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/baseasset"
	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/internal/environmentcopy"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/snapshotrestore"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

func TestRealIncusSnapshotAggregateE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_SNAPSHOT_AGGREGATE") != "1" {
		t.Skip("set HACO_E2E_SNAPSHOT_AGGREGATE=1 on a dedicated root Incus/Btrfs host with cached image/pool")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("root and explicit pool/full image required")
	}
	// Public export now streams and independently verifies the full rootfs in addition
	// to the existing snapshot/restore/copy CLI checks. The dedicated run reached
	// the former eight-minute fixture limit after export and restore had passed.
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	random := func() string { var v [16]byte; _, err := rand.Read(v[:]); must(err); return hex.EncodeToString(v[:]) }
	name := "aggregate-" + random()[:16]
	native := "haco-" + name
	id, err := core.NewEnvironmentInstanceID()
	must(err)
	r := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
	r.setRootPool(pool)
	observed, imageErr := r.runner.Run(ctx, "incus", "query", "/1.0/images/"+image+"?project="+r.project)
	var cached struct{ Fingerprint, Type string }
	if imageErr != nil || observed.ExitCode != 0 || observed.StdoutTruncated || json.Unmarshal([]byte(observed.Stdout), &cached) != nil || cached.Fingerprint != image || cached.Type != "container" {
		t.Fatal("specified cached Base unavailable; no fixture resources created")
	}
	// This fixture has a private catalog and unique resources. Its lifecycle
	// locks must not share the ordinary-user CLI test's temporary directory.
	// Keep production ownership checks and the durable recovery catalog intact.
	t.Setenv("TMPDIR", t.TempDir())
	dir, err := os.MkdirTemp("/var/lib", "haco-snapshot-aggregate-")
	must(err)
	t.Logf("fixture %s; durable failure recovery directory %s", native, dir)
	store := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json"))
	command := func(binary string, args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, binary, args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatalf("%s fixture command failed: %v", binary, err)
		}
		return strings.TrimSpace(out.Stdout)
	}
	collection := gitrepo.Object{Kind: "work", ID: name, Owner: random(), State: "ready"}
	for _, repo := range []string{"one", "two"} {
		collection.Members = append(collection.Members, gitrepo.Object{Kind: "work", ID: name + "-" + repo, Repository: repo, Remote: "https://github.com/example/" + repo + ".git", Branch: "main", Owner: random(), NativeRef: pool + "/haco-work-" + name + "-" + repo, State: "ready"})
	}
	resource := core.PersistentResource{ID: "oci:" + name, Owner: random(), Kind: OCIStoreKind, State: "creating", CreatedAt: time.Now().UTC()}
	resource.NativeRef = pool + "/haco-persistent-" + resource.Owner
	fixture, err := json.Marshal(struct {
		Instance, InstanceID string
		Workspace            gitrepo.Object
		OCI                  core.PersistentResource
	}{native, id, collection, resource})
	must(err)
	f, err := os.OpenFile(filepath.Join(dir, "fixture.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	_, err = f.Write(fixture)
	must(err)
	must(f.Sync())
	must(f.Close())
	base := core.BaseRef{Name: "fixture/base", Revision: core.BaseRevision("sha256:" + image)}
	baseProvider, err := NewBaseProvider(r)
	must(err)
	baseProvider.sources[base.Name] = "local:" + image
	baseBackend := &BaseAssetBackend{Provider: baseProvider}
	baseService := baseasset.Service{Store: store, Backend: baseBackend, Provider: environmentapp.ProviderIncus}
	asset, err := baseService.Ensure(ctx, base, r.project+"/"+pool)
	must(err)
	baseIdentity, _, err := baseBackend.decode(asset)
	must(err)

	persistent := &PersistentResourceBackend{Runtime: r, ImportRoot: dir, ImportLimit: 4 << 30}
	must(store.BeginPersistentResourceCreate(ctx, resource))
	must(persistent.Create(ctx, resource))
	must(persistent.Verify(ctx, resource))
	must(store.CommitPersistentResourceCreate(ctx, resource))
	lease := core.WorkspaceLease{InstanceID: id, EnvironmentID: name, WorkspaceID: core.WorkspaceID(name), SourcePath: "managed:" + name, Owner: name, AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), PersistentResource: resource.Ref()}
	must(store.BeginEnvironmentCreate(ctx, lease))
	command("incus", "init", "local:"+image, native, "--project", r.project, "--no-profiles", "--storage", pool, "--config", environmentInstanceKey+"="+id, "--config", managedEnvironmentMarkerKey+"="+managedEnvironmentMarkerValue)
	lease.RuntimeRef = native
	must(store.RecordEnvironmentRuntime(ctx, lease))
	repository := &RepositoryBackend{Runtime: r, ImportRoot: dir, ImportLimit: 4 << 30}
	for _, member := range collection.Members {
		must(repository.CreateVolume(ctx, member, nil))
	}
	r.ConfigureManagedWorkspaces(func(ctx context.Context, path string) ([]WorkspaceAttachment, error) {
		if path != "managed:"+name {
			return nil, core.ErrInvalidArgument
		}
		return repository.WorkspaceAttachments(ctx, collection)
	})
	mounts, err := repository.WorkspaceAttachments(ctx, collection)
	must(err)
	for _, m := range mounts {
		command("incus", "config", "device", "add", native, m.Device, "disk", "pool="+m.Pool, "source="+m.Volume, "path="+m.Path, "--project", r.project)
	}
	command("incus", "config", "device", "add", native, "persistent-resource", "disk", "pool="+pool, "source=haco-persistent-"+resource.Owner, "path="+OCIStorePath, "--project", r.project)
	volumePath := func(name string) string {
		return filepath.Join("/var/lib/incus/storage-pools", pool, "custom", r.project+"_"+name)
	}
	rootPath := func(name string) string {
		return filepath.Join("/var/lib/incus/storage-pools", pool, "containers", r.project+"_"+name, "rootfs")
	}
	write := func(base, path, value string) {
		t.Helper()
		info, err := os.Lstat(base)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("owned fixture directory unavailable", err)
		}
		must(os.MkdirAll(filepath.Dir(filepath.Join(base, path)), 0700))
		must(os.WriteFile(filepath.Join(base, path), []byte(value), 0600))
	}
	commits := map[string]string{}
	for _, m := range mounts {
		path := volumePath(m.Volume)
		write(path, "tracked", "committed "+m.Device)
		command("git", "-C", path, "init", "--quiet")
		command("git", "-C", path, "add", "tracked")
		command("git", "-C", path, "-c", "user.name=Snapshot Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--quiet", "-m", "fixture")
		commits[m.Device] = command("git", "-C", path, "rev-parse", "HEAD")
		write(path, "tracked", "uncommitted "+m.Device)
		write(path, "untracked", "untracked "+m.Device)
	}
	write(volumePath("haco-persistent-"+resource.Owner), "containerd/data", "actual stored bytes")
	write(volumePath("haco-persistent-"+resource.Owner), "docker/volumes/data", "persistent volume bytes")
	write(filepath.Join(rootPath(native), "root"), "snapshot-marker", "guest-only bytes")
	write(filepath.Join(rootPath(native), "root"), ".ssh/authorized_keys", "ssh-ed25519 AAAA user-key\nssh-ed25519 BBBB haco:ssh-old-generation\n")
	command("sync")
	env := core.Environment{Name: name, RuntimeRef: native, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, AccessMode: lease.AccessMode, Base: &core.BaseRef{Name: "fixture/base", Revision: core.BaseRevision("sha256:" + image)}, PersistentResource: resource.Ref(), CreatedAt: lease.AcquiredAt}
	lease.State = core.WorkspaceLeaseActive
	must(store.CommitEnvironmentCreate(ctx, env, lease))
	router, err := environmentapp.NewRouter(environmentapp.ProviderIncus, environmentapp.Register(environmentapp.ProviderIncus, r))
	must(err)
	runtime := environmentapp.NewBaseRouter(router)
	service := workspace.New(runtime, store)
	if os.Getenv("HACO_E2E_SNAPSHOT_DELETE_IMAGE") == "1" {
		var project struct {
			Name   string
			Config map[string]string
		}
		must(json.Unmarshal([]byte(command("incus", "query", "/1.0/projects/"+r.project)), &project))
		if project.Name != r.project || project.Config["features.images"] != "true" {
			t.Fatal("image project isolation unproven")
		}
		must(baseBackend.Verify(ctx, asset))
		must(r.VerifyEnvironmentIdentity(ctx, native, id))
		var inventory []snapshotInstanceObservation
		must(json.Unmarshal([]byte(command("incus", "query", "/1.0/instances?project="+r.project+"&recursion=1")), &inventory))
		if inventory == nil {
			t.Fatal("instance inventory unproven")
		}
		seen := map[string]bool{}
		for _, instance := range inventory {
			if instance.Name == "" || seen[instance.Name] || instance.Config == nil || instance.ExpandedConfig == nil {
				t.Fatal("instance inventory invalid")
			}
			seen[instance.Name] = true
			if instance.Name != native && instance.Name != baseIdentity.target() && (instance.Config["volatile.base_image"] == image || instance.ExpandedConfig["volatile.base_image"] == image) {
				t.Fatal("another instance depends on selected image")
			}
		}
		if !seen[native] || !seen[baseIdentity.target()] {
			t.Fatal("fixture ownership absent")
		}
		command("incus", "image", "delete", image, "--project", r.project)
		var images []struct{ Fingerprint string }
		must(json.Unmarshal([]byte(command("incus", "query", "/1.0/images?project="+r.project+"&recursion=1")), &images))
		if images == nil {
			t.Fatal("image absence unproven")
		}
		for _, item := range images {
			if item.Fingerprint == image {
				t.Fatal("source image remains")
			}
		}
		t.Log("source image deleted; capture must use only current rootfs and data")
	} else {
		t.Log("SKIP source image deletion: dedicated-image permission not enabled")
	}
	// Prove both capture and restore need neither the original Base nor image.
	must(r.deleteBaseStorage(ctx, baseIdentity))
	snap, err := service.CaptureSnapshot(ctx, name)
	must(err)
	if snap.State != "ready" || len(snap.Components) != 4 {
		t.Fatal("incomplete aggregate", snap.ID, snap.State, len(snap.Components))
	}
	t.Logf("ready snapshot %s with %d components", snap.ID, len(snap.Components))
	reopened := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json"))
	snap, err = reopened.GetSnapshot(ctx, snap.ID)
	must(err)
	read := func(base, path, want string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(base, path))
		must(err)
		if string(data) != want {
			t.Fatalf("saved %s changed", path)
		}
	}
	service = workspace.New(runtime, reopened)
	// Reuse the ordinary canonical routed catalog with native Incus producers.
	// The Base filesystem was already removed; the complete stopped aggregate is
	// the export source, without a separate user snapshot operation.
	exporter := environmenttransfer.Exporter{Snapshots: service, Root: dir, Component: runtime.ExportSnapshotComponent, Workspaces: runtime.ExportSnapshotWorkspaces}
	var readExport func() io.Reader
	var manifest environmenttransfer.Manifest
	if exportCLI := os.Getenv("HACO_E2E_SNAPSHOT_CLI"); exportCLI != "" {
		func() {
			server := control.NewServer()
			must(controlapi.RegisterEnvironmentExport(server, func(ctx context.Context, source string) (environmenttransfer.ExportResult, error) {
				return exporter.ExportStopped(ctx, source, 4<<30)
			}))
			socket := filepath.Join(dir, "export.sock")
			listener, err := control.ListenUnix(socket, 0600)
			must(err)
			serveCtx, stop := context.WithCancel(ctx)
			done := make(chan error, 1)
			go func() { done <- server.Serve(serveCtx, listener) }()
			defer func() { stop(); <-done }()
			t.Setenv("HACO_CONTROL_SOCKET", socket)
			cmd := exec.CommandContext(ctx, exportCLI, "env", "export", "--json", name)
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("public export CLI failed: %v: %s", err, output)
			}
			var receipt struct {
				File   string                             `json:"file"`
				Result controlapi.EnvironmentExportResult `json:"result"`
			}
			must(json.Unmarshal(output, &receipt))
			if receipt.File != name+".haco" || receipt.Result.Bytes <= 0 || receipt.Result.TemporarySnapshot != "" {
				t.Fatal("incomplete public export receipt", receipt)
			}
		}()
		file, err := os.Open(filepath.Join(dir, name+".haco"))
		must(err)
		defer file.Close()
		info, err := file.Stat()
		must(err)
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			t.Fatal("public export permissions", info.Mode())
		}
		readExport = func() io.Reader { return io.NewSectionReader(file, 0, info.Size()) }
		manifest, err = environmenttransfer.Inspect(readExport(), 4<<30)
		must(err)
		t.Log("PASS shipped haco env export with source only, private Unix controller stream, default .haco file, complete verified bundle and no required snapshot command")
	} else {
		exported, err := exporter.ExportStopped(ctx, name, 4<<30)
		must(err)
		if exported.Bundle == nil || exported.TemporarySnapshot != "" {
			t.Fatal("aggregate export incomplete", exported.TemporarySnapshot)
		}
		defer exported.Bundle.Close()
		manifest = exported.Bundle.Manifest()
		readExport = exported.Bundle.Reader
		t.Log("SKIP public export CLI: HACO_E2E_SNAPSHOT_CLI not supplied; internal native export ran")
	}
	if manifest.Version != 2 || manifest.Source != name || !manifest.HasOCI || len(manifest.Components) != 4 || len(manifest.Workspaces) != 2 {
		t.Fatal("aggregate export omitted managed data", manifest)
	}
	for _, w := range manifest.Workspaces {
		matched := false
		for _, member := range collection.Members {
			if member.Repository == w.Name && member.Remote == w.Remote && member.Branch == w.Branch {
				matched = true
			}
		}
		if !matched {
			t.Fatal("export routing differs from protected Workspace", w)
		}
	}
	t.Log("PASS canonical routed native export: rootfs, both Git Workspace volumes and OCI; no Base filesystem; temporary capture cleaned before bundle return")
	// Change current work after saving, then prove preparation preserves it and
	// stages the earlier saved bytes. This does not publish or start a replacement.
	write(filepath.Join(rootPath(native), "root"), "snapshot-marker", "changed current work")
	for _, m := range mounts {
		write(volumePath(m.Volume), "tracked", "changed "+m.Device)
		write(volumePath(m.Volume), "untracked", "changed untracked "+m.Device)
	}
	write(volumePath("haco-persistent-"+resource.Owner), "containerd/data", "changed containerd")
	write(volumePath("haco-persistent-"+resource.Owner), "docker/volumes/data", "changed Docker volume")
	command("sync")
	prepared, err := service.PrepareSnapshotRestore(ctx, name, snap.ID)
	must(err)
	if prepared.State != "prepared" || len(prepared.Components) != 4 {
		t.Fatal("restore staging incomplete")
	}
	prepared, err = reopened.GetSnapshotRestore(ctx, prepared.ID)
	must(err)
	for _, component := range prepared.Components {
		must(runtime.VerifyRestoreComponent(ctx, component))
		prefix := "haco-runtime-v1:" + environmentapp.ProviderIncus + ":"
		if !strings.HasPrefix(component.NativeRef, prefix) {
			t.Fatal("restore route absent")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(component.NativeRef, prefix))
		must(err)
		local := component
		local.NativeRef = string(decoded)
		binding, err := r.decodeRestore(local)
		must(err)
		source, _, _, instance, err := r.restoreShape(binding)
		must(err)
		switch {
		case component.Role == "rootfs":
			read(rootPath(binding.target()), "root/snapshot-marker", "guest-only bytes")
			write(filepath.Join(rootPath(binding.target()), "root"), "snapshot-marker", "edited staged rootfs")
		case !instance && component.Role == "oci":
			read(volumePath(binding.target()), "containerd/data", "actual stored bytes")
			read(volumePath(binding.target()), "docker/volumes/data", "persistent volume bytes")
			write(volumePath(binding.target()), "containerd/data", "edited staged OCI")
		case !instance:
			path := volumePath(binding.target())
			device := source.Volume.Device
			if command("git", "-C", path, "rev-parse", "HEAD") != commits[device] {
				t.Fatal("staged Git commit missing")
			}
			read(path, "tracked", "uncommitted "+device)
			read(path, "untracked", "untracked "+device)
			if _, err := os.Lstat(filepath.Join(path, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
				t.Fatal("staged Git depends on source objects", err)
			}
			write(path, "tracked", "edited staged work")
		}
	}
	if prepared.Before.ID != "" {
		t.Fatal("restore made an automatic backup")
	}
	rawCatalog, err := os.ReadFile(filepath.Join(dir, "state.json"))
	must(err)
	var inventoryCatalog struct {
		Snapshots map[string]core.Snapshot `json:"snapshots"`
	}
	must(json.Unmarshal(rawCatalog, &inventoryCatalog))
	if len(inventoryCatalog.Snapshots) != 1 {
		t.Fatal("unexpected automatic snapshot")
	}
	read(rootPath(native), "root/snapshot-marker", "changed current work")
	for _, m := range mounts {
		read(volumePath(m.Volume), "tracked", "changed "+m.Device)
		read(volumePath(m.Volume), "untracked", "changed untracked "+m.Device)
	}
	read(volumePath("haco-persistent-"+resource.Owner), "containerd/data", "changed containerd")
	read(volumePath("haco-persistent-"+resource.Owner), "docker/volumes/data", "changed Docker volume")
	must(service.CleanupSnapshotRestore(ctx, prepared.ID))
	t.Log("PASS four-component restore preparation without Base or automatic backup, durable reload, saved rootfs/Git/OCI bytes staged, current work unchanged, staging edits independent, owned staging cleanup; no Environment replacement performed")
	must(r.VerifyEnvironmentIdentity(ctx, native, id))
	must(service.Delete(ctx, name))
	if _, err := environmenttransfer.Inspect(readExport(), 4<<30); err != nil {
		t.Fatal("exported bundle changed after source mutation/deletion", err)
	}
	t.Log("PASS exported native aggregate remains complete after source Env deletion; public import/SSH not asserted by export")
	if exists, err := r.environmentExists(ctx, native); err != nil || exists {
		t.Fatal("source instance absence unproven", err)
	}
	// All source-volume identities were recorded before creation; delete only those
	// with exact ownership and positively observed no remaining attachments.
	for _, m := range mounts {
		p := snapshotVolumePlan{Pool: m.Pool, Source: m.Volume, SourceOwner: m.Owner, SourceKind: "work", SourceID: m.Repository, SourceInstance: native, SourceInstanceID: id, Owner: random(), Role: "workspace:" + strings.TrimPrefix(m.Volume, "haco-work-")}
		observed, err := r.snapshotVolumeObservation(ctx, p, false)
		must(err)
		if observed == nil || len(observed.UsedBy) != 0 {
			t.Fatal("source volume ownership/absence scope")
		}
		command("incus", "storage", "volume", "delete", pool, m.Volume, "--project", r.project)
		observed, err = r.snapshotVolumeObservation(ctx, p, false)
		must(err)
		if observed != nil {
			t.Fatal("source volume remains")
		}
	}
	deleting, err := reopened.BeginPersistentResourceDelete(ctx, resource.ID)
	must(err)
	must(persistent.Delete(ctx, deleting))
	must(reopened.FinalizePersistentResourceDelete(ctx, deleting))

	// The original runtime and volumes are absent. Register normal managed copies
	// using only the saved aggregate, then reopen the registry and inspect Git data.
	restoredRepositories := gitrepo.NewRepositoryService(dir, repository)
	restoredRepositories.SnapshotCatalog = store
	restoredWork, err := restoredRepositories.RestoreWorkspace(ctx, "restored-"+strings.TrimPrefix(name, "aggregate-"), snap)
	must(err)
	reopenedRepositories := gitrepo.NewRepositoryService(dir, repository)
	reloadedWork, err := reopenedRepositories.Get("work", restoredWork.ID)
	must(err)
	if reloadedWork.Owner != restoredWork.Owner || reloadedWork.RestoredFrom != snap.ID {
		t.Fatal("restored ownership lost")
	}
	restoredMounts, err := repository.WorkspaceAttachments(ctx, reloadedWork)
	must(err)
	for _, m := range restoredMounts {
		path := volumePath(m.Volume)
		if command("git", "-C", path, "rev-parse", "HEAD") != commits[m.Device] {
			t.Fatal("restored unpushed commit lost")
		}
		read(path, "tracked", "uncommitted "+m.Device)
		read(path, "untracked", "untracked "+m.Device)
		write(path, "untracked", "independent registered copy")
	}

	restoredStores := persistentresource.Service{Store: reopened, Backend: persistent}
	restoredOCI, err := restoredStores.RestoreSnapshot(ctx, "oci:restored-"+strings.TrimPrefix(name, "aggregate-"), snap, core.WorkspaceID("workspace:managed:"+reloadedWork.Owner))
	must(err)
	_, restoredOCIVolume, err := persistentVolume(restoredOCI)
	must(err)
	read(volumePath(restoredOCIVolume), "containerd/data", "actual stored bytes")
	read(volumePath(restoredOCIVolume), "docker/volumes/data", "persistent volume bytes")
	write(volumePath(restoredOCIVolume), "containerd/data", "independent registered OCI copy")
	reopenedOCI, err := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json")).GetPersistentResource(ctx, restoredOCI.ID)
	must(err)
	if reopenedOCI != restoredOCI || reopenedOCI.RestoreSource != "" || reopenedOCI.State != "ready" {
		t.Fatal("OCI publication receipt drift")
	}
	// Reuse the deleted source name through the canonical service. The saved
	// snapshot still describes the old generation; new permissions must not.
	resumedName := name
	resumedPath := "managed:" + reloadedWork.ID
	r.ConfigureManagedWorkspaces(func(ctx context.Context, path string) ([]WorkspaceAttachment, error) {
		if !strings.HasPrefix(path, "managed:") {
			return nil, core.ErrInvalidArgument
		}
		object, err := reopenedRepositories.Get("work", strings.TrimPrefix(path, "managed:"))
		if err != nil {
			return nil, err
		}
		return repository.WorkspaceAttachments(ctx, object)
	})
	sandbox, err := NewSandboxProvider(r)
	must(err)
	resumedRouter, err := environmentapp.NewRouter(environmentapp.ProviderIncus, environmentapp.Register(environmentapp.ProviderIncus, sandbox))
	must(err)
	resumedService := workspace.NewWithProvider(environmentapp.NewBaseRouter(resumedRouter), reopened, aggregateWorkspaceResolver{reopenedRepositories})
	resumedEnv, err := resumedService.CreateFromSnapshot(ctx, core.EnvironmentSpec{Name: resumedName, WorkspacePath: resumedPath, PersistentResource: restoredOCI.ID}, snap.ID)
	must(err)
	resumedID, err := reopened.EnvironmentInstance(ctx, resumedEnv)
	must(err)
	resumedLease, err := reopened.GetWorkspaceLease(ctx, resumedName)
	must(err)
	if resumedLease.SnapshotSource != "" || resumedLease.State != core.WorkspaceLeaseActive {
		t.Fatal("saved source reservation not released on publication")
	}
	resumed := core.EnvironmentRuntime{Ref: "haco-" + resumedName}
	must(r.VerifyEnvironmentIdentity(ctx, resumed.Ref, resumedID))
	if err := r.VerifyEnvironmentIdentity(ctx, resumed.Ref, id); err == nil {
		t.Fatal("old generation accepted")
	}
	if got := command("incus", "exec", resumed.Ref, "--project", r.project, "--", "cat", "/root/snapshot-marker"); got != "guest-only bytes" {
		t.Fatal("rootfs data lost")
	}
	if got := command("incus", "exec", resumed.Ref, "--project", r.project, "--", "cat", "/root/.ssh/authorized_keys"); got != "ssh-ed25519 AAAA user-key" {
		t.Fatal("managed SSH authorization inherited", got)
	}
	for _, m := range restoredMounts {
		if got := command("incus", "exec", resumed.Ref, "--project", r.project, "--", "cat", m.Path+"/tracked"); got != "uncommitted "+m.Device {
			t.Fatal("Workspace data unavailable")
		}
	}
	if got := command("incus", "exec", resumed.Ref, "--project", r.project, "--", "cat", OCIStorePath+"/containerd/data"); got != "independent registered OCI copy" {
		t.Fatal("OCI data unavailable")
	}
	// Exercise the shipped CLI through a private real controller socket, with
	// real Incus stopped copies and automatic restart of this running runtime.
	binary := os.Getenv("HACO_E2E_SNAPSHOT_CLI")
	var cliSavedID string
	if binary != "" {
		func() {
			server := control.NewServer()
			must(controlapi.RegisterManagedWorkspaces(server, resumedService))
			must(controlapi.RegisterSnapshots(server, resumedService))
			must(controlapi.RegisterSnapshotRestore(server, &snapshotrestore.Service{Catalog: reopened, Environments: resumedService, Workspaces: restoredRepositories, Stores: &restoredStores}))
			must(controlapi.RegisterEnvironmentCopy(server, &environmentcopy.Service{Catalog: reopened, Snapshots: resumedService, Restorer: &snapshotrestore.Service{Catalog: reopened, Environments: resumedService, Workspaces: restoredRepositories, Stores: &restoredStores}}))
			socket := filepath.Join(dir, "cli.sock")
			listener, err := control.ListenUnix(socket, 0600)
			must(err)
			serveCtx, stopServer := context.WithCancel(ctx)
			done := make(chan error, 1)
			go func() { done <- server.Serve(serveCtx, listener) }()
			defer func() { stopServer(); <-done }()
			t.Setenv("HACO_CONTROL_SOCKET", socket)
			refused := exec.CommandContext(ctx, binary, "workspace", "delete", "--yes", reloadedWork.ID)
			if output, err := refused.CombinedOutput(); err == nil || !strings.Contains(string(output), "referenced by an Environment") {
				t.Fatalf("attached Workspace deletion not refused: %v %s", err, output)
			}

			invoke := func(args ...string) []controlapi.SnapshotSummary {
				t.Helper()
				cmd := exec.CommandContext(ctx, binary, args...)
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("snapshot CLI %v failed: %v: %s", args, err, output)
				}
				var summaries []controlapi.SnapshotSummary
				must(json.Unmarshal(output, &summaries))
				return summaries
			}
			saved := invoke("snapshot", "create", "--json", resumedName)
			if len(saved) != 1 || saved[0].State != "ready" || saved[0].Workspaces != 2 || !saved[0].OCI {
				t.Fatal("incomplete CLI save", saved)
			}
			cliSavedID = saved[0].ID
			t.Logf("public CLI saved %s from running %s", cliSavedID, resumedName)
			current, err := r.InspectEnvironment(ctx, resumed.Ref)
			must(err)
			if current.State != core.EnvironmentRunning {
				t.Fatal("source not resumed", current)
			}
			listed := invoke("snapshot", "list", "--json", resumedName)
			found := false
			for _, item := range listed {
				if item.ID == cliSavedID {
					found = true
				}
			}
			if !found {
				t.Fatal("CLI save missing from list", listed)
			}
		}()
	} else {
		t.Log("SKIP public snapshot CLI: HACO_E2E_SNAPSHOT_CLI binary not supplied")
	}
	must(resumedService.Delete(ctx, resumedName))
	if exists, err := r.environmentExists(ctx, resumed.Ref); err != nil || exists {
		t.Fatal("restored runtime absence unproven", err)
	}
	// Exercise the normal router and canonical archive creation against the
	// already exported bundle after deleting both prior executing Environments.
	// All data bindings are independently imported from the verified bundle. Public
	// CLI transport and an SSH handshake are separate acceptance requirements.
	func() {
		importer := environmenttransfer.Importer{Catalog: reopened, Environments: resumedService, Workspaces: restoredRepositories, Stores: &restoredStores, Root: dir, StoreKind: OCIStoreKind}
		receipt, err := importer.Import(ctx, readExport(), resumedName, 4<<30)
		must(err)
		if receipt.State != "running" || receipt.Workspace == reloadedWork.ID || receipt.OCI == restoredOCI.ID {
			t.Fatal("bundle reused existing data", receipt)
		}
		imported, err := reopened.GetEnvironment(ctx, receipt.Environment)
		must(err)
		importedWork, err := restoredRepositories.Get("work", receipt.Workspace)
		must(err)
		importedOCI, err := reopened.GetPersistentResource(ctx, receipt.OCI)
		must(err)
		if importedOCI.WorkspaceID != imported.Workspace.ID || importedOCI.Owner == restoredOCI.Owner || importedWork.Owner == reloadedWork.Owner {
			t.Fatal("import data association/owner lost")
		}
		importedMounts, err := repository.WorkspaceAttachments(ctx, importedWork)
		must(err)
		importedID, err := reopened.EnvironmentInstance(ctx, imported)
		must(err)
		if imported.Base != nil || importedID == resumedID || importedID == id {
			t.Fatal("archive import adopted Base or generation")
		}
		must(r.VerifyEnvironmentIdentity(ctx, resumed.Ref, importedID))
		for _, old := range []string{id, resumedID} {
			if err := r.VerifyEnvironmentIdentity(ctx, resumed.Ref, old); err == nil {
				t.Fatal("archive import accepted old identity")
			}
		}
		if command("incus", "exec", resumed.Ref, "--project", r.project, "--", "cat", "/root/snapshot-marker") != "guest-only bytes" {
			t.Fatal("imported rootfs bytes lost")
		}
		if command("incus", "exec", resumed.Ref, "--project", r.project, "--", "cat", "/root/.ssh/authorized_keys") != "ssh-ed25519 AAAA user-key" {
			t.Fatal("imported managed SSH authority retained")
		}
		for _, m := range importedMounts {
			if command("incus", "exec", resumed.Ref, "--project", r.project, "--", "cat", m.Path+"/tracked") != "uncommitted "+m.Device {
				t.Fatal("imported Workspace binding lost")
			}
		}
		if command("incus", "exec", resumed.Ref, "--project", r.project, "--", "cat", OCIStorePath+"/containerd/data") != "actual stored bytes" {
			t.Fatal("imported OCI binding lost")
		}
		receipts, err := filepath.Glob(filepath.Join(dir, "rootfs-import-*.jsonl"))
		if err != nil || len(receipts) != 0 {
			t.Fatal("temporary image cleanup incomplete", err)
		}
		must(resumedService.Delete(ctx, resumedName))
		must(persistent.Verify(ctx, importedOCI))
		for _, m := range importedMounts {
			read(volumePath(m.Volume), "untracked", "untracked "+m.Device)
			path := volumePath(m.Volume)
			if command("git", "-c", "safe.directory="+path, "-C", path, "rev-parse", "HEAD") != commits[m.Device] {
				t.Fatal("import lost unpushed commit")
			}
		}
		if _, err := environmenttransfer.Inspect(readExport(), 4<<30); err != nil {
			t.Fatal("source bundle changed", err)
		}
		must(resumedService.CleanupRestoredData(ctx, imported.Workspace, func(ctx context.Context) error {
			if err := restoredStores.DeleteRestoredCopy(ctx, importedOCI); err != nil {
				return err
			}
			return restoredRepositories.DeleteWorkspace(ctx, importedWork.ID, importedWork.Owner)
		}))
		t.Log("PASS canonical bundle import: normal router, real running Env, fresh generation, current sandbox/managed SSH reset, independently imported Workspace/OCI, no Base, temporary image cleanup and retained data after Env deletion; public import and SSH handshake not tested")
	}()
	if cliSavedID != "" {
		saved, err := reopened.GetSnapshot(ctx, cliSavedID)
		must(err)
		for _, component := range saved.Components {
			must(resumedRouter.VerifySnapshotComponent(ctx, component))
		}
		server := control.NewServer()
		must(controlapi.RegisterSnapshots(server, resumedService))
		must(controlapi.RegisterSnapshotRestore(server, &snapshotrestore.Service{Catalog: reopened, Environments: resumedService, Workspaces: restoredRepositories, Stores: &restoredStores}))
		must(controlapi.RegisterEnvironmentCopy(server, &environmentcopy.Service{Catalog: reopened, Snapshots: resumedService, Restorer: &snapshotrestore.Service{Catalog: reopened, Environments: resumedService, Workspaces: restoredRepositories, Stores: &restoredStores}}))
		socket := filepath.Join(dir, "delete.sock")
		listener, err := control.ListenUnix(socket, 0600)
		must(err)
		serveCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- server.Serve(serveCtx, listener) }()
		t.Setenv("HACO_CONTROL_SOCKET", socket)
		listedOutput, listErr := exec.CommandContext(ctx, binary, "snapshot", "list", "--json", resumedName).CombinedOutput()
		var listed []controlapi.SnapshotSummary
		if listErr != nil || json.Unmarshal(listedOutput, &listed) != nil {
			stop()
			<-done
			t.Fatalf("list after source deletion: %v: %s", listErr, listedOutput)
		}
		found := false
		for _, item := range listed {
			if item.ID == cliSavedID {
				found = true
			}
		}
		if !found {
			stop()
			<-done
			t.Fatal("save disappeared with source", listed)
		}
		restoreOutput, restoreErr := exec.CommandContext(ctx, binary, "snapshot", "restore", "--json", cliSavedID, resumedName).CombinedOutput()
		var restored snapshotrestore.Result
		if restoreErr != nil || json.Unmarshal(restoreOutput, &restored) != nil || restored.State != "running" {
			stop()
			<-done
			t.Fatalf("public restore: %v: %s", restoreErr, restoreOutput)
		}
		publicEnv, err := reopened.GetEnvironment(ctx, restored.Environment)
		must(err)
		publicGeneration, err := reopened.EnvironmentInstance(ctx, publicEnv)
		must(err)
		if publicGeneration == resumedID || publicGeneration == id {
			t.Fatal("public restore reused permission generation")
		}
		publicWork, err := restoredRepositories.Get("work", restored.Workspace)
		must(err)
		publicOCI, err := reopened.GetPersistentResource(ctx, restored.OCI)
		must(err)
		publicMounts, err := repository.WorkspaceAttachments(ctx, publicWork)
		must(err)
		for _, m := range publicMounts {
			if command("incus", "exec", "haco-"+resumedName, "--project", r.project, "--", "cat", m.Path+"/tracked") != "uncommitted "+m.Device {
				t.Fatal("public restored Workspace lost data")
			}
		}
		if command("incus", "exec", "haco-"+resumedName, "--project", r.project, "--", "cat", "/root/snapshot-marker") != "guest-only bytes" {
			t.Fatal("public rootfs lost")
		}
		if command("incus", "exec", "haco-"+resumedName, "--project", r.project, "--", "cat", OCIStorePath+"/containerd/data") != "independent registered OCI copy" {
			t.Fatal("public OCI lost")
		}
		t.Logf("public restore running %s with Workspace %s and OCI %s", restored.Environment, restored.Workspace, restored.OCI)

		copyName := resumedName + "-copy"
		beforeCopies, e := resumedService.ListSnapshots(ctx, "")
		must(e)
		if output, e := exec.CommandContext(ctx, binary, "env", "copy", "--json", resumedName, copyName).CombinedOutput(); e == nil {
			t.Fatalf("running copy accepted: %s", output)
		}
		afterCopies, e := resumedService.ListSnapshots(ctx, "")
		must(e)
		if len(afterCopies) != len(beforeCopies) {
			t.Fatal("running copy created a save")
		}
		must(resumedService.Stop(ctx, resumedName))
		copyOutput, copyErr := exec.CommandContext(ctx, binary, "env", "copy", "--json", resumedName, copyName).CombinedOutput()
		var copied environmentcopy.Result
		if copyErr != nil || json.Unmarshal(copyOutput, &copied) != nil || copied.State != "running" || copied.TemporarySnapshot != "" {
			t.Fatalf("public copy: %v: %s", copyErr, copyOutput)
		}
		copiedEnv, e := reopened.GetEnvironment(ctx, copyName)
		must(e)
		copiedGeneration, e := reopened.EnvironmentInstance(ctx, copiedEnv)
		must(e)
		if copiedGeneration == publicGeneration {
			t.Fatal("copy reused generation")
		}
		copiedWork, e := restoredRepositories.Get("work", copied.Workspace)
		must(e)
		copiedOCI, e := reopened.GetPersistentResource(ctx, copied.OCI)
		must(e)
		afterCopies, e = resumedService.ListSnapshots(ctx, "")
		must(e)
		if len(afterCopies) != len(beforeCopies) {
			t.Fatal("copy retained temporary save")
		}
		if command("incus", "exec", "haco-"+copyName, "--project", r.project, "--", "cat", "/root/snapshot-marker") != "guest-only bytes" {
			t.Fatal("copied rootfs lost")
		}
		command("incus", "exec", "haco-"+copyName, "--project", r.project, "--", "/bin/sh", "-c", "printf copied > /root/snapshot-marker")
		sourceStatus, e := r.InspectEnvironment(ctx, "haco-"+resumedName)
		must(e)
		if sourceStatus.State != core.EnvironmentStopped {
			t.Fatal("copy started the source")
		}
		must(resumedService.Start(ctx, resumedName))
		if command("incus", "exec", "haco-"+resumedName, "--project", r.project, "--", "cat", "/root/snapshot-marker") != "guest-only bytes" {
			t.Fatal("copy mutation changed source")
		}
		must(resumedService.Stop(ctx, resumedName))

		cmd := exec.CommandContext(ctx, binary, "snapshot", "delete", cliSavedID)
		output, deleteErr := cmd.CombinedOutput()
		stop()
		<-done
		if deleteErr != nil {
			t.Fatalf("CLI delete: %v: %s", deleteErr, output)
		}
		if _, err := reopened.GetSnapshot(ctx, cliSavedID); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("save not deleted", err)
		}
		must(resumedService.Delete(ctx, publicEnv.Name))
		if command("incus", "exec", "haco-"+copyName, "--project", r.project, "--", "cat", "/root/snapshot-marker") != "copied" {
			t.Fatal("copy depends on deleted source")
		}
		copiedMounts, e := repository.WorkspaceAttachments(ctx, copiedWork)
		must(e)
		for _, m := range copiedMounts {
			if command("incus", "exec", "haco-"+copyName, "--project", r.project, "--", "cat", m.Path+"/tracked") != "uncommitted "+m.Device {
				t.Fatal("copied Work lost")
			}
			copiedPath := volumePath(m.Volume)
			read(copiedPath, "untracked", "independent registered copy")
			if command("git", "-c", "safe.directory="+copiedPath, "-C", copiedPath, "rev-parse", "HEAD") != commits[m.Device] {
				t.Fatal("copied unpushed commit lost")
			}
			command("incus", "exec", "haco-"+copyName, "--project", r.project, "--", "/bin/sh", "-c", `printf copied-work > "$1"`, "--", m.Path+"/tracked")
		}
		if command("incus", "exec", "haco-"+copyName, "--project", r.project, "--", "cat", OCIStorePath+"/containerd/data") != "independent registered OCI copy" {
			t.Fatal("copied OCI lost")
		}
		must(resumedService.Delete(ctx, copyName))
		must(resumedService.CleanupRestoredData(ctx, copiedEnv.Workspace, func(ctx context.Context) error {
			if err := restoredStores.DeleteRestoredCopy(ctx, copiedOCI); err != nil {
				return err
			}
			return restoredRepositories.DeleteRestoredCopy(ctx, copiedWork)
		}))
		t.Log("PASS public Env copy: running source refused, stopped source copied, temporary save removed, fresh generation, independent rootfs/Work/OCI after source deletion, owned cleanup")

		must(persistent.Verify(ctx, publicOCI))
		for _, m := range publicMounts {
			read(volumePath(m.Volume), "tracked", "uncommitted "+m.Device)
		}
		must(resumedService.CleanupRestoredData(ctx, publicEnv.Workspace, func(ctx context.Context) error {
			if err := restoredStores.DeleteRestoredCopy(ctx, publicOCI); err != nil {
				return err
			}
			return restoredRepositories.DeleteRestoredCopy(ctx, publicWork)
		}))
		t.Log("PASS public snapshot create/list/restore/delete: one-command restore after source deletion, fresh generation and independent rootfs/Git/OCI, saved-copy deletion, normal Env deletion retaining data, owned restored-data cleanup; actual restored SSH handshake not tested")
	}
	must(persistent.Verify(ctx, restoredOCI))
	for _, m := range restoredMounts {
		read(volumePath(m.Volume), "tracked", "uncommitted "+m.Device)
	}
	t.Log("PASS canonical saved-rootfs activation without Base, same-name fresh generation/current source guard, guest root/Workspace/OCI bytes, managed SSH authorization reset, canonical runtime deletion retaining data; actual restored SSH handshake not tested")

	if binary != "" {
		func() {
			server := control.NewServer()
			must(controlapi.RegisterManagedWorkspaces(server, resumedService))
			socket := filepath.Join(dir, "workspace-cleanup.sock")
			listener, err := control.ListenUnix(socket, 0600)
			must(err)
			serveCtx, stop := context.WithCancel(ctx)
			done := make(chan error, 1)
			go func() { done <- server.Serve(serveCtx, listener) }()
			defer func() { stop(); <-done }()
			t.Setenv("HACO_CONTROL_SOCKET", socket)
			output, err := exec.CommandContext(ctx, binary, "workspace", "list", "--json").CombinedOutput()
			must(err)
			var listed []workspace.ManagedWorkspace
			must(json.Unmarshal(output, &listed))
			found := false
			for _, w := range listed {
				if w.Name == reloadedWork.ID {
					found = true
					if len(w.Environments) != 0 || len(w.Stores) != 1 || w.Stores[0] != restoredOCI.ID {
						t.Fatal("incorrect retained-data references", w)
					}
				}
			}
			if !found {
				t.Fatal("retained Workspace missing")
			}

			// Native children are not Hacocoon aggregate snapshots. Neither kind may
			// disappear as a side effect of deleting the live Workspace volume.
			member := reloadedWork.Copies()[0]
			pool, volume, err := volumeRef(member)
			must(err)
			nativePath := "/1.0/storage-pools/" + pool + "/volumes/custom/" + volume
			for _, kind := range []string{"snapshots", "backups"} {
				child := "retained-fixture"
				command("incus", "query", "-X", "POST", nativePath+"/"+kind+"?project="+r.project, "--data", `{"name":"retained-fixture"}`, "--wait")
				output, err := exec.CommandContext(ctx, binary, "workspace", "delete", "--yes", reloadedWork.ID).CombinedOutput()
				if err == nil {
					t.Fatalf("native %s unexpectedly deleted: %s", kind, output)
				}
				if record, err := reopenedRepositories.Get("work", reloadedWork.ID); err != nil || record.State != "ready" {
					t.Fatal("native saved-data refusal disabled Workspace", record, err)
				}
				command("incus", "query", nativePath+"/"+kind+"/"+child+"?project="+r.project)
				for _, m := range reloadedWork.Copies() {
					must(repository.InspectVolume(ctx, m))
				}
				command("incus", "query", "-X", "DELETE", nativePath+"/"+kind+"/"+child+"?project="+r.project, "--wait")
			}
			t.Log("PASS native child snapshot/backup refusal before deletion; registry remains ready and every member volume remains; explicit owned child cleanup")
			output, err = exec.CommandContext(ctx, binary, "workspace", "delete", "--yes", reloadedWork.ID).CombinedOutput()
			if err != nil {
				t.Fatalf("Workspace cleanup CLI: %v %s", err, output)
			}
			if _, err := reopenedRepositories.Get("work", reloadedWork.ID); !errors.Is(err, core.ErrNotFound) {
				t.Fatal("Workspace registration remains", err)
			}
			for _, member := range reloadedWork.Copies() {
				pool, name, err := volumeRef(member)
				must(err)
				raw := command("incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+r.project+"&recursion=1")
				var volumes []persistentVolumeObservation
				must(json.Unmarshal([]byte(raw), &volumes))
				if volumes == nil {
					t.Fatal("absence observation missing")
				}
				for _, v := range volumes {
					if v.Name == name {
						t.Fatal("CLI left owned Workspace volume", name)
					}
				}
			}
			must(persistent.Verify(ctx, restoredOCI))
			for _, component := range snap.Components {
				must(runtime.VerifySnapshotComponent(ctx, component))
			}
			t.Log("PASS public Workspace list/delete: attached refusal, retained Git data after Env deletion, exact managed identity, independent snapshot and OCI preservation, all owned member volumes absent")
		}()
	} else {
		must(resumedService.DeleteManagedWorkspace(ctx, resumedEnv.Workspace))
		t.Log("SKIP public Workspace cleanup CLI: HACO_E2E_SNAPSHOT_CLI not supplied; canonical native deletion ran")
	}
	must(restoredStores.DeleteForWorkspace(ctx, restoredOCI.ID, restoredOCI.WorkspaceID))
	t.Log("PASS restored OCI registered with new owner and Workspace, source reservation released, durable reload, saved bytes and independent edits, canonical owned deletion")

	t.Log("PASS normal Workspace registration/reload from saved data after original Env/volumes deletion; Git commits/uncommitted/untracked retained; independent mutation; exact copy cleanup; no Git network operation")

	// Intentionally remove the fixture-owned original Base to prove snapshot independence.
	// Keep its durable receipt until the complete fixture is positively absent.
	must(r.deleteBaseStorage(ctx, baseIdentity))
	for _, component := range snap.Components {
		must(runtime.VerifySnapshotComponent(ctx, component))
		prefix := "haco-runtime-v1:" + environmentapp.ProviderIncus + ":"
		if !strings.HasPrefix(component.NativeRef, prefix) {
			t.Fatal("missing provider route")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(component.NativeRef, prefix))
		must(err)
		local := component
		local.NativeRef = string(decoded)
		binding, err := r.decodeSnapshotComponent(local)
		must(err)
		switch {
		case binding.Rootfs != nil:
			read(rootPath(binding.Rootfs.target()), "root/snapshot-marker", "guest-only bytes")
		case binding.Volume != nil:
			p := binding.Volume
			path := volumePath(p.target())
			if p.Role == "oci" {
				read(path, "containerd/data", "actual stored bytes")
				read(path, "docker/volumes/data", "persistent volume bytes")
			} else {
				if command("git", "-C", path, "rev-parse", "HEAD") != commits[p.Device] {
					t.Fatal("unpushed commit lost")
				}
				read(path, "tracked", "uncommitted "+p.Device)
				read(path, "untracked", "untracked "+p.Device)
				if _, err := os.Lstat(filepath.Join(path, ".git", "objects", "info", "alternates")); !os.IsNotExist(err) {
					t.Fatal("Git depends on source objects", err)
				}
			}
		}
	}
	must(service.DeleteSnapshot(ctx, snap.ID))
	if _, err := reopened.GetSnapshot(ctx, snap.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("snapshot catalog cleanup", err)
	}
	entries, err := os.ReadDir(dir)
	must(err)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			t.Fatal("unexpected recovery entry", entry.Name())
		}
		must(os.Remove(filepath.Join(dir, entry.Name())))
	}
	must(os.Remove(dir))
	t.Log("PASS canonical catalog/coordinator/provider route; complete four-component save; restart readback; source Environment/volumes deleted; independent Git and data retained; owned snapshot cleanup. No snapshot Base material retained; image deletion reported separately; public CLI coverage reported separately; restored SSH handshake/live OCI consistency not tested.")
}

// The fixture uses the same trusted registry lookup as application composition.
type aggregateWorkspaceResolver struct{ repositories *gitrepo.RepositoryService }

func (p aggregateWorkspaceResolver) Resolve(ctx context.Context, req workspace.WorkspaceRequest) (core.Workspace, error) {
	if !strings.HasPrefix(req.Path, "managed:") {
		return core.Workspace{}, core.ErrInvalidArgument
	}
	return p.repositories.Workspace(ctx, strings.TrimPrefix(req.Path, "managed:"))
}

func (p aggregateWorkspaceResolver) ListManagedWorkspaces(ctx context.Context) ([]workspace.ManagedWorkspace, error) {
	objects, err := p.repositories.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	result := []workspace.ManagedWorkspace{}
	for _, o := range objects {
		repos := []string{}
		for _, m := range o.Copies() {
			repos = append(repos, m.Repository)
		}
		result = append(result, workspace.ManagedWorkspace{Name: o.ID, State: o.State, Repositories: repos, Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + o.Owner), Path: "managed:" + o.ID}})
	}
	return result, nil
}
func (p aggregateWorkspaceResolver) DeleteWorkspace(ctx context.Context, w core.Workspace) error {
	return p.repositories.DeleteWorkspace(ctx, strings.TrimPrefix(w.Path, "managed:"), strings.TrimPrefix(string(w.ID), "workspace:managed:"))
}
