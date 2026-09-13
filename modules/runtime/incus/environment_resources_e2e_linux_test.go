//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
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
)

// This exercises ordinary Env creation, two rootfs cache placements and canonical
// resume/delete. It does not claim selected-path collection or large-repo speed.
func TestRealIncusEnvironmentDataPlacementE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS") != "1" {
		t.Skip("set HACO_E2E_INCUS=1 on an Incus host")
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
	selected := []core.EnvironmentResourceSelection{}
	for _, key := range []string{"compiler", "packages"} {
		origin, err := store.EnsureResourceGeneration(ctx, key, CacheResourceKind, strings.Repeat("a", 64))
		must(err)
		selected = append(selected, core.EnvironmentResourceSelection{Key: key, Target: "/root/.cache/haco-e2e-" + key, Origin: origin})
	}
	svc.ConfigureEnvironmentResources(resources, func(context.Context, core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		return selected, nil
	})
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
	must(svc.Delete(ctx, name))
	cleaned = true
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
	t.Log("PASS ordinary creation, two writable rootfs areas, exact resume/client resume, native target drift refusal, disposable cleanup and Workspace retention")
}
