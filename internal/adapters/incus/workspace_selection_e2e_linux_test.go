//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
)

func TestRealIncusWorkspaceSelectionE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_WORKSPACE_SELECTION") != "1" {
		t.Skip("set HACO_E2E_WORKSPACE_SELECTION=1 on an installed dedicated Incus Host")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("root, explicit pool and full cached image required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	random := func() string { var b [16]byte; _, err := rand.Read(b[:]); must(err); return hex.EncodeToString(b[:]) }
	name := "selection-" + random()[:12]
	dir, err := os.MkdirTemp("/var/lib", "haco-selection-")
	must(err)
	t.Logf("owned fixture=%s catalog=%s; failure retains Workspace records", name, filepath.Join(dir, "state.json"))
	r := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
	r.setRootPool(pool)
	must(r.verifyTrustedHostOwnership(ctx))
	backend := &RepositoryBackend{Runtime: r, ImportRoot: dir, ImportLimit: 4 << 30}
	store := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json"))
	repos := gitrepo.NewRepositoryService(dir, backend)
	repos.SnapshotCatalog = store
	collection := gitrepo.Object{Kind: "work", ID: name, Owner: random(), State: "ready"}
	for _, key := range []string{"one", "two"} {
		collection.Members = append(collection.Members, gitrepo.Object{Kind: "work", ID: name + "-" + key, Owner: random(), State: "ready", Repository: key, Remote: "https://github.com/example/" + key + ".git", Branch: "main", NativeRef: pool + "/haco-work-" + name + "-" + key})
	}
	third := gitrepo.Object{Kind: "repo", ID: name + "-third", Repository: name + "-third", Owner: random(), State: "ready", Remote: "https://github.com/example/third.git", Branch: "main", NativeRef: pool + "/haco-repo-" + name + "-third"}
	write := func(path string, value any) {
		t.Helper()
		f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		must(e)
		must(json.NewEncoder(f).Encode(value))
		must(f.Sync())
		must(f.Close())
	}
	// All fixture identities precede allocation. These virgin volumes contain
	// test-authored Git data only; no guest-written work is mounted in the Host.
	write(filepath.Join(dir, "fixture.json"), []gitrepo.Object{collection, third})
	run := func(args ...string) string {
		t.Helper()
		out, e := r.runner.Run(ctx, "incus", args...)
		must(e)
		if out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatal("native fixture command incomplete")
		}
		return out.Stdout
	}
	for _, object := range append(collection.Copies(), third) {
		must(backend.CreateVolume(ctx, object, nil))
		must(backend.InspectVolume(ctx, object))
		root := gitadapter.WorkspaceRoot
		if object.Kind == "repo" {
			root = gitadapter.RepositoryRoot
		}
		path := root + "/" + object.ID
		device := "haco-" + object.Kind + "-" + object.ID
		run("config", "device", "add", trustedHostName, device, "disk", "pool="+pool, "source="+strings.TrimPrefix(object.NativeRef, pool+"/"), "path="+path, "--project", r.project)
		run("exec", trustedHostName, "--project", r.project, "--", "sh", "-ceu", `cd "$1"
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
git init --template= --initial-branch=main
printf original > tracked
git add -- tracked
git -c user.name=Fixture -c user.email=fixture@example.invalid commit -m initial
printf staged > tracked
git add -- tracked
printf dirty > tracked
printf untracked > extra
`, "--", path)
		if object.Kind == "work" {
			run("config", "device", "remove", trustedHostName, device, "--project", r.project)
		}
	}
	write(filepath.Join(dir, "repo-"+third.ID+".json"), third)
	r.ConfigureManagedWorkspaces(func(ctx context.Context, path string) ([]WorkspaceAttachment, error) {
		if path == "managed:"+name {
			return backend.WorkspaceAttachments(ctx, collection)
		}
		object, e := repos.Get("work", strings.TrimPrefix(path, "managed:"))
		if e != nil {
			return nil, e
		}
		return backend.WorkspaceAttachments(ctx, object)
	})
	p, err := NewSandboxProvider(r)
	must(err)
	p.sources["fixture-parent"] = "local:" + image
	resolver := savedDataWorkspaceResolver{source: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + collection.Owner), Path: "managed:" + name}, repositories: repos}
	svc := workspace.NewWithProvider(p, store, resolver)
	owned := map[string]bool{}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 45*time.Second)
		defer stop()
		for id := range owned {
			if e := svc.Delete(cleanup, id); e != nil {
				t.Errorf("owned Env cleanup: %v", e)
			}
		}
	}()
	env, err := svc.Create(ctx, core.EnvironmentSpec{Name: name, WorkspacePath: resolver.source.Path, Base: "fixture-parent", SkipDefaultResource: true})
	must(err)
	owned[name] = true
	read := func(ref, path string) string {
		return run("exec", ref, "--project", r.project, "--", "sh", "-ceu", `cd "$1"; cat tracked extra .git/HEAD; sha256sum .git/index`, "--", path)
	}
	original := read(env.RuntimeRef, "/workspace/one")
	must(svc.Stop(ctx, name))
	saved, err := svc.CaptureStoppedSnapshotForWorkspace(ctx, name, resolver.source.ID)
	must(err)
	copied, err := repos.RestoreWorkspaceSelectionWithData(ctx, name+"-next", saved, []string{"one", third.ID}, nil)
	must(err)
	must(svc.DeleteSnapshot(ctx, saved.ID))
	destination, err := svc.Create(ctx, core.EnvironmentSpec{Name: name + "-next", WorkspacePath: "managed:" + copied.ID, ExpectedWorkspace: core.WorkspaceID("workspace:managed:" + copied.Owner), Base: "fixture-parent", SkipDefaultResource: true})
	must(err)
	owned[destination.Name] = true
	if read(destination.RuntimeRef, "/workspace/one") != original {
		t.Fatal("saved dirty files, index or HEAD changed")
	}
	if !strings.Contains(read(destination.RuntimeRef, "/workspace/"+third.ID), "dirtyuntracked") {
		t.Fatal("added repository data unavailable")
	}
	run("exec", destination.RuntimeRef, "--project", r.project, "--", "sh", "-ceu", `test ! -e /workspace/two; printf independent > /workspace/one/tracked`)
	must(svc.Start(ctx, name))
	if read(env.RuntimeRef, "/workspace/one") != original {
		t.Fatal("destination edited source")
	}
	run("exec", env.RuntimeRef, "--project", r.project, "--", "test", "-f", "/workspace/two/tracked")
	must(svc.Delete(ctx, name))
	delete(owned, name)
	must(svc.Stop(ctx, destination.Name))
	must(svc.Start(ctx, destination.Name))
	if !strings.Contains(read(destination.RuntimeRef, "/workspace/one"), "independentuntracked") {
		t.Fatal("copy depends on source Env")
	}
	must(svc.Delete(ctx, destination.Name))
	delete(owned, destination.Name)
	must(svc.CleanupRestoredData(ctx, destination.Workspace, func(ctx context.Context) error { return repos.DeleteRestoredCopy(ctx, copied) }))
	for _, member := range collection.Copies() {
		must(backend.DeleteWorkspaceVolume(ctx, member))
	}
	if os.Getenv("HACO_E2E_WORKSPACE_INPUT") == "1" {
		nativeWorkspaceInput(t, ctx, svc, repos, dir, name, third.ID, run)
	}
	must(repos.DeleteSource(ctx, third.ID, third.Owner))
	t.Log("PASS selected saved dirty/index/HEAD data, registered addition, omitted member retained at source, independent edits, source Env deletion, restart and exact cleanup; OCI, authenticated Git and large-repository performance not exercised")
}
