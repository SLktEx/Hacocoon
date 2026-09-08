package incus

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/internal/host"
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
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
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
	r := New(host.ExecRunner{})
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
		collection.Members = append(collection.Members, gitrepo.Object{Kind: "work", ID: name + "-" + repo, Repository: repo, Owner: random(), NativeRef: pool + "/haco-work-" + name + "-" + repo, State: "ready"})
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
	persistent := &PersistentResourceBackend{Runtime: r}
	must(store.BeginPersistentResourceCreate(ctx, resource))
	must(persistent.Create(ctx, resource))
	must(persistent.Verify(ctx, resource))
	must(store.CommitPersistentResourceCreate(ctx, resource))
	lease := core.WorkspaceLease{InstanceID: id, EnvironmentID: name, WorkspaceID: core.WorkspaceID(name), SourcePath: "managed:" + name, Owner: name, AccessMode: core.WorkspaceReadWrite, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now().UTC(), PersistentResource: resource.Ref()}
	must(store.BeginEnvironmentCreate(ctx, lease))
	command("incus", "init", "local:"+image, native, "--project", r.project, "--no-profiles", "--storage", pool, "--config", environmentInstanceKey+"="+id, "--config", managedEnvironmentMarkerKey+"="+managedEnvironmentMarkerValue)
	lease.RuntimeRef = native
	must(store.RecordEnvironmentRuntime(ctx, lease))
	repository := &RepositoryBackend{Runtime: r}
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
	command("sync")
	env := core.Environment{Name: name, RuntimeRef: native, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: lease.SourcePath}, AccessMode: lease.AccessMode, Base: &core.BaseRef{Name: "fixture/base", Revision: core.BaseRevision("sha256:" + image)}, PersistentResource: resource.Ref(), CreatedAt: lease.AcquiredAt}
	lease.State = core.WorkspaceLeaseActive
	must(store.CommitEnvironmentCreate(ctx, env, lease))
	router, err := environmentapp.NewRouter(environmentapp.ProviderIncus, environmentapp.Register(environmentapp.ProviderIncus, r))
	must(err)
	runtime := environmentapp.NewBaseRouter(router)
	service := workspace.New(runtime, store)
	snap, err := service.CaptureSnapshot(ctx, name)
	must(err)
	if snap.State != "ready" || len(snap.Components) != 5 {
		t.Fatal("incomplete aggregate", snap.ID, snap.State, len(snap.Components))
	}
	t.Logf("ready snapshot %s with %d components", snap.ID, len(snap.Components))
	reopened := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json"))
	snap, err = reopened.GetSnapshot(ctx, snap.ID)
	must(err)
	service = workspace.New(runtime, reopened)
	must(r.VerifyEnvironmentIdentity(ctx, native, id))
	must(service.Delete(ctx, name))
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
	read := func(base, path, want string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(base, path))
		must(err)
		if string(data) != want {
			t.Fatalf("saved %s changed", path)
		}
	}
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
	t.Log("PASS canonical catalog/coordinator/provider route; complete five-component save; restart readback; source Environment/volumes deleted; independent Git and data retained; owned snapshot cleanup. Shared Base image retained; restore/live OCI consistency not tested.")
}
