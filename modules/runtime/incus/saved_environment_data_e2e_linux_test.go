//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmentcopy"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/snapshotrestore"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

func TestRealIncusSavedEnvironmentDataE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_SAVED_DATA") != "1" {
		t.Skip("set HACO_E2E_SAVED_DATA=1 with an explicit Incus pool and cached image")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("root, pool and full cached image required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	random := func() string { var b [16]byte; _, err := rand.Read(b[:]); must(err); return hex.EncodeToString(b[:]) }
	name := "saved-data-" + random()[:16]
	dir, err := os.MkdirTemp("/var/lib", "haco-saved-data-")
	must(err)
	t.Logf("owned fixture=%s catalog=%s; incomplete operations retain recovery receipts", name, filepath.Join(dir, "state.json"))
	r := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
	r.setRootPool(pool)
	p, err := NewSandboxProvider(r)
	must(err)
	p.sources["fixture-parent"] = "local:" + image
	store := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json"))
	backend := &RepositoryBackend{Runtime: r}
	repositories := gitrepo.NewRepositoryService(dir, backend)
	repositories.SnapshotCatalog = store
	collection := gitrepo.Object{ID: name, Kind: "work", Owner: random(), State: "ready"}
	for _, key := range []string{"one", "two"} {
		collection.Members = append(collection.Members, gitrepo.Object{ID: name + "-" + key, Kind: "work", Owner: random(), State: "ready", Repository: key, Remote: "https://github.com/example/" + key + ".git", Branch: "main", NativeRef: pool + "/haco-work-" + name + "-" + key})
	}
	// Fixture identity precedes native allocation; failures keep this recovery file.
	f, err := os.OpenFile(filepath.Join(dir, "fixture.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	must(json.NewEncoder(f).Encode(collection))
	must(f.Sync())
	must(f.Close())
	for _, member := range collection.Members {
		must(backend.CreateVolume(ctx, member, nil))
		must(backend.InspectVolume(ctx, member))
	}
	r.ConfigureManagedWorkspaces(func(ctx context.Context, path string) ([]WorkspaceAttachment, error) {
		if path == "managed:"+name {
			return backend.WorkspaceAttachments(ctx, collection)
		}
		object, err := repositories.Get("work", strings.TrimPrefix(path, "managed:"))
		if err != nil {
			return nil, err
		}
		return backend.WorkspaceAttachments(ctx, object)
	})
	resolver := savedDataWorkspaceResolver{source: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + collection.Owner), Path: "managed:" + name}, repositories: repositories}
	svc := workspace.NewWithProvider(p, store, resolver)
	resources := &persistentresource.Service{Store: store, Backend: &PersistentResourceBackend{Runtime: r}}
	selected := []core.EnvironmentResourceSelection{}
	packagesPath := "/root/.cache/saved-packages"
	if os.Getenv("HACO_E2E_SAVED_DATA_PLACEMENT") == "repository" {
		packagesPath = "/workspace/two/node_modules"
	}
	for _, pair := range [][2]string{{"compiler", "/root/.cache/saved-compiler"}, {"packages", packagesPath}} {
		origin, err := store.EnsureResourceGeneration(ctx, pair[0], CacheResourceKind, strings.Repeat("a", 64))
		must(err)
		selected = append(selected, core.EnvironmentResourceSelection{Key: pair[0], Target: pair[1], Origin: origin})
	}
	svc.ConfigureEnvironmentResources(resources, func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		return selected, nil
	})
	owned := map[string]bool{}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 60*time.Second)
		defer stop()
		for id := range owned {
			if err := svc.Delete(cleanup, id); err != nil {
				t.Errorf("owned Env cleanup: %v", err)
			}
		}
	}()
	env, err := svc.Create(ctx, core.EnvironmentSpec{Name: name, WorkspacePath: resolver.source.Path, Base: "fixture-parent", SkipDefaultResource: true})
	must(err)
	owned[name] = true
	run := func(args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "incus", args...)
		must(err)
		if out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatal("native command incomplete")
		}
		return out.Stdout
	}
	run("exec", env.RuntimeRef, "--project", r.project, "--", "sh", "-ceu", `printf compiler-bytes > /root/.cache/saved-compiler/probe; printf packages-bytes > "$1/probe"; printf work-bytes > /workspace/one/probe`, "--", packagesPath)
	// Running capture must resume with the complete data binding.
	saved, err := svc.CaptureSnapshot(ctx, name)
	must(err)
	if len(saved.Components) != 5 {
		t.Fatal("incomplete saved aggregate")
	}
	status, err := p.InspectEnvironment(ctx, env.RuntimeRef)
	must(err)
	if status.State != core.EnvironmentRunning {
		t.Fatal("capture did not resume")
	}
	must(svc.DeleteSnapshot(ctx, saved.ID))
	must(svc.Stop(ctx, name))
	restorer := &snapshotrestore.Service{Catalog: store, Environments: svc, Workspaces: repositories, Stores: resources}
	copier := &environmentcopy.Service{Catalog: store, Snapshots: svc, Restorer: restorer}
	result, err := copier.CopyEnvironment(ctx, name, name+"-copy")
	must(err)
	owned[result.Environment] = true
	if result.State != "running" || result.TemporarySnapshot != "" {
		t.Fatal("copy incomplete", result)
	}
	copied, err := store.GetEnvironment(ctx, result.Environment)
	must(err)
	if len(copied.Attachments) != 2 {
		t.Fatal("copied data missing")
	}
	for i, a := range copied.Attachments {
		if a.Resource == env.Attachments[i].Resource || a.Origin != env.Attachments[i].Origin {
			t.Fatal("copy ownership/provenance changed")
		}
	}
	read := func(ref string) string {
		return run("exec", ref, "--project", r.project, "--", "cat", "/root/.cache/saved-compiler/probe", packagesPath+"/probe", "/workspace/one/probe")
	}
	if read(copied.RuntimeRef) != "compiler-bytespackages-byteswork-bytes" {
		t.Fatal("uncollected data lost")
	}
	run("exec", copied.RuntimeRef, "--project", r.project, "--", "sh", "-ceu", "printf changed > /root/.cache/saved-compiler/probe")
	must(svc.Start(ctx, name))
	if read(env.RuntimeRef) != "compiler-bytespackages-byteswork-bytes" {
		t.Fatal("copy mutation changed source")
	}
	must(svc.Delete(ctx, name))
	delete(owned, name)
	if read(copied.RuntimeRef) != "changedpackages-byteswork-bytes" {
		t.Fatal("copy depends on deleted source")
	}
	must(svc.Stop(ctx, copied.Name))
	must(svc.Start(ctx, copied.Name))
	if read(copied.RuntimeRef) != "changedpackages-byteswork-bytes" {
		t.Fatal("copy resume lost data")
	}
	must(svc.Delete(ctx, copied.Name))
	delete(owned, copied.Name)
	for _, e := range []core.Environment{env, copied} {
		for _, a := range e.Attachments {
			if _, err := store.GetPersistentResource(ctx, a.Resource.ID); !errors.Is(err, core.ErrNotFound) {
				t.Fatal("child retained", err)
			}
		}
	}
	work, err := repositories.Get("work", result.Workspace)
	must(err)
	must(svc.CleanupRestoredData(ctx, core.Workspace{ID: core.WorkspaceID("workspace:managed:" + work.Owner), Path: "managed:" + work.ID}, func(ctx context.Context) error { return repositories.DeleteRestoredCopy(ctx, work) }))
	for _, member := range collection.Members {
		must(backend.DeleteWorkspaceVolume(ctx, member))
	}
	t.Logf("PASS running snapshot/resume, stopped copy with compiler and packages (%s) data, fresh child ownership, uncollected bytes, independent mutation/source deletion, copied resume and canonical owned cleanup; no performance claim", packagesPath)
}

type savedDataWorkspaceResolver struct {
	source       core.Workspace
	repositories *gitrepo.RepositoryService
}

func (p savedDataWorkspaceResolver) Resolve(ctx context.Context, req workspace.WorkspaceRequest) (core.Workspace, error) {
	if req.Path == p.source.Path {
		return p.source, nil
	}
	if !strings.HasPrefix(req.Path, "managed:") {
		return core.Workspace{}, core.ErrInvalidArgument
	}
	return p.repositories.Workspace(ctx, strings.TrimPrefix(req.Path, "managed:"))
}
