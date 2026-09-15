//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	cacheapp "github.com/SLktEx/Hacocoon/modules/standard/cache"
)

// This exercises ordinary Env creation, two rootfs cache placements and canonical
// resume/delete and whole-generation collection/reuse. It does not measure large-repo speed.
func TestRealIncusEnvironmentDataPlacementE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_RESUME") != "1" {
		t.Skip("set HACO_E2E_INCUS_RESUME=1 on an Incus host")
	}
	pool, image := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE")
	if !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) {
		t.Fatal("explicit pool and full cached image fingerprint required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var nonce [8]byte
	_, err := rand.Read(nonce[:])
	must(err)
	name := "data-e2e-" + hex.EncodeToString(nonce[:])
	root, err := os.MkdirTemp("/var/lib", "haco-data-placement-")
	must(err)
	work := filepath.Join(root, "work")
	must(os.Mkdir(work, 0755))
	t.Logf("owned Env=%s catalog=%s; ambiguous cleanup retains ownership", name, filepath.Join(root, "state.json"))
	r := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
	r.setRootPool(pool)
	p, err := NewSandboxProvider(r)
	must(err)
	p.sources["fixture-parent"] = "local:" + image
	store := state.NewEnvironmentJSONStore(filepath.Join(root, "state.json"))
	resources := &persistentresource.Service{Store: store, Backend: &PersistentResourceBackend{Runtime: r}}
	svc := workspace.New(p, store)
	configuration := cacheapp.Configuration{}
	for _, key := range []string{"compiler", "packages"} {
		configuration.Areas = append(configuration.Areas, cacheapp.Area{Name: key, Path: "/root/.cache/haco-e2e-" + key, Compatibility: "fixture-format-1"})
	}
	settings := cacheapp.Settings{Path: filepath.Join(root, "cache.json")}
	initial, err := settings.Read(ctx)
	must(err)
	initial.Configuration = configuration
	_, err = settings.Replace(ctx, initial)
	must(err)
	svc.ConfigureEnvironmentResources(resources, func(ctx context.Context, request core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		selector, err := settings.Select(ctx, store, nil)
		if err != nil {
			return nil, err
		}
		return selector.Select(ctx, request)
	})
	workflow := &cacheapp.Workflow{Settings: settings, Catalog: store, Collector: svc, Cleaner: resources}
	cleaned, createdOK := false, false
	defer func() {
		// Create owns failed-creation cleanup. A name collision is not authority
		// for this fixture to delete an already-existing native Environment.
		if cleaned || !createdOK {
			return
		}
		cleanup, stop := context.WithTimeout(context.Background(), 45*time.Second)
		defer stop()
		if err := svc.Delete(cleanup, name); err != nil && !errors.Is(err, core.ErrNotFound) {
			t.Errorf("owned cleanup requires recovery: %v", err)
		}
	}()
	env, err := svc.Create(ctx, core.EnvironmentSpec{Name: name, WorkspacePath: work, Base: "fixture-parent", SkipDefaultResource: true})
	must(err)
	createdOK = true
	if len(env.Attachments) != 2 {
		t.Fatal("missing disposable data")
	}
	run := func(args ...string) string {
		t.Helper()
		out, err := r.runner.Run(ctx, "incus", args...)
		must(err)
		if out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatal("native command incomplete")
		}
		return out.Stdout
	}
	run("exec", env.RuntimeRef, "--project", r.project, "--", "sh", "-ceu", "printf compiler-data > /root/.cache/haco-e2e-compiler/probe; printf package-data > /root/.cache/haco-e2e-packages/probe; printf keep-work > /workspace/probe")
	must(svc.Stop(ctx, name))
	if err := p.StartEnvironment(ctx, env.RuntimeRef); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("reference-only resume accepted", err)
	}
	must(svc.Start(ctx, name))
	got := run("exec", env.RuntimeRef, "--project", r.project, "--", "cat", "/root/.cache/haco-e2e-compiler/probe", "/root/.cache/haco-e2e-packages/probe", "/workspace/probe")
	if got != "compiler-datapackage-datakeep-work" {
		t.Fatal("data changed across resume")
	}
	must(svc.Stop(ctx, name))
	// Simulate a native device edit on this fixture, confirm normal resume refuses
	// it, then restore only the exact test-owned device configuration.
	device := environmentDataDevicePrefix + env.Attachments[0].Key
	run("config", "device", "set", env.RuntimeRef, device, "path=/root/.ssh", "--project", r.project)
	if err := svc.Start(ctx, name); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatal("wrong target accepted", err)
	}
	run("config", "device", "set", env.RuntimeRef, device, "path="+env.Attachments[0].Target, "--project", r.project)
	must(svc.Start(ctx, name))
	// Stop-triggered client access must use the same complete resource binding.
	must(svc.Stop(ctx, name))
	must(svc.WithClientAccess(ctx, name, nil, true, nil, func(core.Environment, string) error { return nil }))
	must(svc.Stop(ctx, name))
	collected, err := workflow.Collect(ctx, name, "")
	must(err)
	if len(collected) != 2 || collected[0].State != "published" || collected[1].State != "published" {
		t.Fatal("ordinary collection incomplete", collected)
	}
	must(svc.Delete(ctx, name))
	cleaned = true
	p.sources["fixture-next"] = "local:" + image
	nextName := name + "-next"
	next, err := svc.Create(ctx, core.EnvironmentSpec{Name: nextName, WorkspacePath: work, Base: "fixture-next", SkipDefaultResource: true})
	must(err)
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 45*time.Second)
		defer stop()
		if err := svc.Delete(cleanup, nextName); err != nil && !errors.Is(err, core.ErrNotFound) {
			t.Errorf("consumer cleanup: %v", err)
		}
	}()
	reused := run("exec", next.RuntimeRef, "--project", r.project, "--", "cat", "/root/.cache/haco-e2e-compiler/probe", "/root/.cache/haco-e2e-packages/probe", "/workspace/probe")
	if reused != "compiler-datapackage-datakeep-work" {
		t.Fatal("whole-generation reuse lost contents")
	}
	for i, a := range next.Attachments {
		if a.Origin.Number != 1 || a.Resource == env.Attachments[i].Resource {
			t.Fatal("consumer did not receive independent generation copy")
		}
	}
	run("exec", next.RuntimeRef, "--project", r.project, "--", "sh", "-ceu", "printf changed > /root/.cache/haco-e2e-compiler/probe")
	// Review and clear source generations while independent consumer data stays live.
	for _, a := range next.Attachments {
		if os.Getenv("HACO_E2E_CACHE_CATALOG") == "1" && a.Key == "packages" {
			continue
		}
		history, err := workflow.History(ctx, nextName, a.Key)
		must(err)
		if len(history.Entries) != 1 || history.Entries[0].State != "current" {
			t.Fatal("missing named source history", history)
		}
		cleared, err := workflow.Clear(ctx, nextName, a.Key, history.Revision)
		must(err)
		if !cleared.Reset || len(cleared.Entries) != 1 || cleared.Entries[0].State != "deleted" {
			t.Fatal("source cleanup incomplete", cleared)
		}
	}
	if got := run("exec", next.RuntimeRef, "--project", r.project, "--", "cat", "/root/.cache/haco-e2e-compiler/probe", "/root/.cache/haco-e2e-packages/probe", "/workspace/probe"); got != "changedpackage-datakeep-work" {
		t.Fatal("clear changed consumer data")
	}
	must(svc.Delete(ctx, nextName))
	if os.Getenv("HACO_E2E_CACHE_CATALOG") == "1" {
		history, err := workflow.CatalogHistory(ctx)
		must(err)
		if len(history.Groups) != 2 {
			t.Fatal("retained catalog incomplete", history)
		}
		count := 0
		for _, group := range history.Groups {
			if len(group.Environments) != 0 {
				t.Fatal("producer should be gone", group)
			}
			count += len(group.History.Entries)
		}
		if count != 1 {
			t.Fatal("missing orphaned collected data", history)
		}
		cleared, err := workflow.MaintainCatalog(ctx, history.Revision, true)
		must(err)
		for _, group := range cleared.Groups {
			if group.State != "complete" || !group.Result.Reset {
				t.Fatal("orphan cleanup incomplete", group)
			}
			for _, entry := range group.Result.Entries {
				if entry.State != "deleted" {
					t.Fatal(entry)
				}
			}
		}
		t.Log("PASS reviewed catalog cleanup of retained collected data after all producer/consumer Envs were deleted, through exact ownership and common deletion; Workspace retained")
	}
	for _, a := range env.Attachments {
		if _, err := store.GetPersistentResource(ctx, a.Resource.ID); !errors.Is(err, core.ErrNotFound) {
			t.Fatal("child ownership was not released", err)
		}
	}
	data, err := os.ReadFile(filepath.Join(work, "probe"))
	must(err)
	if string(data) != "keep-work" {
		t.Fatal("Workspace was not retained")
	}
	t.Log("PASS ordinary creation, two writable rootfs areas, exact resume/client resume, native target drift refusal, disposable cleanup, Host settings, stopped collection, data-bearing independent reuse with another Base name, and Workspace retention")
}
