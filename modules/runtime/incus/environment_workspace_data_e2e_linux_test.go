//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type environmentDataWorkspaceFixture struct{ work core.Workspace }

func (f environmentDataWorkspaceFixture) Resolve(_ context.Context, request workspace.WorkspaceRequest) (core.Workspace, error) {
	if request.Path != f.work.Path {
		return core.Workspace{}, core.ErrInvalidArgument
	}
	return f.work, nil
}

// Native placement/lifetime acceptance with private owned Workspace fixtures.
// It is not public Host selection, cache collection or large-repository evidence.
func TestRealIncusEnvironmentWorkspaceDataPlacementE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_RESUME") != "1" {
		t.Skip("set HACO_E2E_INCUS_RESUME=1 on an Incus host")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("explicit pool and full cached image required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	r := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
	r.setRootPool(pool)
	connect, err := localDaemonConnect(r.project)
	must(err)
	server, err := connect(ctx)
	must(err)
	defer server.Disconnect()
	if !server.HasExtension("file_storage_volume") {
		t.Fatal("required file_storage_volume API unavailable; no fixture resources created")
	}
	connection, err := server.GetConnectionInfo()
	must(err)
	random := func() string { var b [16]byte; _, err := rand.Read(b[:]); must(err); return hex.EncodeToString(b[:]) }
	name := "repo-data-" + random()[:16]
	dir, err := os.MkdirTemp("/var/lib", "haco-repository-data-")
	must(err)
	collection := gitrepo.Object{ID: name, Kind: "work", Owner: random(), State: "ready"}
	for _, member := range []string{"one", "two"} {
		id := name + "-" + member
		collection.Members = append(collection.Members, gitrepo.Object{ID: id, Kind: "work", Owner: random(), State: "ready", Repository: member, NativeRef: pool + "/haco-work-" + id})
	}
	// Exact fixture identities are durable before requesting native volumes.
	receipt, err := os.OpenFile(filepath.Join(dir, "fixture.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	must(json.NewEncoder(receipt).Encode(collection))
	must(receipt.Sync())
	must(receipt.Close())
	t.Logf("owned fixture=%s catalog=%s; failed placement retains recovery records", name, filepath.Join(dir, "state.json"))
	repository := &RepositoryBackend{Runtime: r}
	for _, member := range collection.Members {
		must(repository.CreateVolume(ctx, member, nil))
		must(repository.InspectVolume(ctx, member))
	}
	r.ConfigureManagedWorkspaces(func(ctx context.Context, source string) ([]WorkspaceAttachment, error) {
		if source != "managed:"+name {
			return nil, core.ErrInvalidArgument
		}
		return repository.WorkspaceAttachments(ctx, collection)
	})
	p, err := NewSandboxProvider(r)
	must(err)
	p.sources["fixture-parent"] = "local:" + image
	store := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json"))
	resources := &persistentresource.Service{Store: store, Backend: &PersistentResourceBackend{Runtime: r}}
	resolver := environmentDataWorkspaceFixture{core.Workspace{ID: core.WorkspaceID("workspace:managed:" + collection.Owner), Path: "managed:" + name}}
	svc := workspace.NewWithProvider(p, store, resolver)
	selected := []core.EnvironmentResourceSelection{}
	for _, pair := range [][2]string{{"compiler", "/workspace/one/build/cache"}, {"packages", "/workspace/two/node_modules"}} {
		origin, err := store.EnsureResourceGeneration(ctx, pair[0], CacheResourceKind, strings.Repeat("a", 64))
		must(err)
		selected = append(selected, core.EnvironmentResourceSelection{Key: pair[0], Target: pair[1], Origin: origin})
	}
	svc.ConfigureEnvironmentResources(resources, func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		return selected, nil
	})
	owned := map[string]bool{}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 45*time.Second)
		defer stop()
		for id := range owned {
			if err := svc.Delete(cleanup, id); err != nil {
				t.Errorf("owned Env cleanup requires recovery: %v", err)
			}
		}
		// Workspace fixtures are deleted only on the positively verified success
		// path below. Failed/uncertain creates must keep their data and receipts.
	}()
	spec := core.EnvironmentSpec{Name: name, WorkspacePath: resolver.work.Path, Base: "fixture-parent", SkipDefaultResource: true}
	env, err := svc.Create(ctx, spec)
	must(err)
	owned[name] = true
	run := func(args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "incus", args...)
		must(err)
		if out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatal("fixture command incomplete")
		}
		return out.Stdout
	}
	run("exec", env.RuntimeRef, "--project", r.project, "--", "sh", "-ceu", "printf compiler > /workspace/one/build/cache/probe; printf packages > /workspace/two/node_modules/probe; printf keep-one > /workspace/one/probe; printf keep-two > /workspace/two/probe")
	must(svc.Stop(ctx, name))
	must(svc.Start(ctx, name))
	if got := run("exec", env.RuntimeRef, "--project", r.project, "--", "cat", "/workspace/one/build/cache/probe", "/workspace/two/node_modules/probe"); got != "compilerpackages" {
		t.Fatal("cache changed across resume")
	}
	must(svc.Stop(ctx, name))
	// A same-Env but different Workspace volume is not an acceptable parent.
	run("config", "device", "set", env.RuntimeRef, "workspace-one", "source="+strings.TrimPrefix(collection.Members[1].NativeRef, pool+"/"), "--project", r.project)
	if err := svc.Start(ctx, name); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("parent substitution accepted", err)
	}
	run("config", "device", "set", env.RuntimeRef, "workspace-one", "source="+strings.TrimPrefix(collection.Members[0].NativeRef, pool+"/"), "--project", r.project)
	must(svc.WithClientAccess(ctx, name, nil, true, nil, func(core.Environment, string) error { return nil }))
	must(svc.Delete(ctx, name))
	delete(owned, name)
	for _, a := range env.Attachments {
		if _, err := store.GetPersistentResource(ctx, a.Resource.ID); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("disposable resource retained", err)
		}
	}
	// Reuse the retained Workspace through ordinary creation with no cache areas.
	retained := workspace.NewWithProvider(p, store, resolver)
	spec.Name = name + "-retained"
	reused, err := retained.Create(ctx, spec)
	must(err)
	owned[spec.Name] = true
	if got := run("exec", reused.RuntimeRef, "--project", r.project, "--", "cat", "/workspace/one/probe", "/workspace/two/probe"); got != "keep-onekeep-two" {
		t.Fatal("Workspace data lost")
	}
	run("exec", reused.RuntimeRef, "--project", r.project, "--", "sh", "-ceu", "test ! -e /workspace/one/build/cache/probe; test ! -e /workspace/two/node_modules/probe; ln -s /root/.ssh /workspace/one/cache-link")
	must(retained.Delete(ctx, spec.Name))
	delete(owned, spec.Name)
	// An ordinary guest supplied the link. Collection/placement may not follow
	// it, erase it, or publish a partially created Environment.
	selected[0].Target = "/workspace/one/cache-link/cache"
	spec.Name = name + "-unsafe"
	_, err = svc.Create(ctx, spec)
	if err == nil {
		owned[spec.Name] = true
		t.Fatal("guest link accepted")
	}
	if !errors.Is(err, core.ErrIncompatibleState) || errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal("unexpected link refusal/cleanup", err)
	}
	if _, err := store.GetWorkspaceLease(ctx, spec.Name); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("failed placement lease retained", err)
	}
	// Read only the known fixture bytes through the native volume API, proving
	// failure cleanup did not remove the Workspace data or the guest's link.
	observe := func(method, path string) *http.Response {
		u, err := url.Parse(connection.URL)
		must(err)
		u.Path = "/1.0/storage-pools/" + pool + "/volumes/custom/" + strings.TrimPrefix(collection.Members[0].NativeRef, pool+"/") + "/files"
		u.RawQuery = url.Values{"project": {r.project}, "path": {path}}.Encode()
		req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
		must(err)
		response, err := server.DoHTTP(req)
		must(err)
		if response.StatusCode != http.StatusOK {
			must(response.Body.Close())
			t.Fatal("retained fixture unavailable")
		}
		return response
	}
	response := observe(http.MethodGet, "/probe")
	data, err := io.ReadAll(io.LimitReader(response.Body, 64))
	must(response.Body.Close())
	must(err)
	if string(data) != "keep-one" {
		t.Fatal("failure changed unconfigured data")
	}
	response = observe(http.MethodHead, "/cache-link")
	must(response.Body.Close())
	if response.Header.Get("X-Incus-type") != "symlink" {
		t.Fatal("failure changed guest link")
	}
	for _, member := range collection.Members {
		must(repository.DeleteWorkspaceVolume(ctx, member))
	}
	t.Log("PASS two managed repositories, native file inspection, cache persistence, parent drift refusal, client resume, canonical disposable deletion, retained Workspace reuse and guest-link refusal with data preserved")
}
