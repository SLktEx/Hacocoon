package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"strings"
	"testing"
)

// This adapter exercises real runtime formats/removal within the existing
// isolated copy fixture. Canonical generation locking is tested in workspace;
// this is not a claim of full installed-controller acceptance.
type runtimeImageFixture struct {
	runtime  *Runtime
	name     string
	resource core.PersistentResource
	instance string
}

func (f *runtimeImageFixture) GetEnvironment(context.Context, string) (core.Environment, error) {
	return core.Environment{Name: f.name, PersistentResource: f.resource.Ref()}, nil
}
func (f *runtimeImageFixture) EnvironmentInstance(context.Context, core.Environment) (string, error) {
	return f.instance, nil
}
func (f *runtimeImageFixture) GetPersistentResource(context.Context, string) (core.PersistentResource, error) {
	return f.resource, nil
}
func (f *runtimeImageFixture) ListSnapshots(context.Context) ([]core.Snapshot, error) {
	return nil, nil
}
func (f *runtimeImageFixture) ExecForResource(ctx context.Context, name, instance string, ref core.PersistentResourceRef, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if name != f.name || instance != f.instance || ref != f.resource.Ref() {
		return core.ExecutionResult{}, core.ErrCapabilityStale
	}
	args := append([]string{"exec", f.name, "--project", f.runtime.project, "--"}, req.Argv...)
	out, err := f.runtime.runner.Run(ctx, "incus", args...)
	return core.ExecutionResult{Stdout: out.Stdout, Stderr: out.Stderr, ExitCode: out.ExitCode, StdoutTruncated: out.StdoutTruncated, StderrTruncated: out.StderrTruncated}, err
}
func verifyManagedRuntimeImages(t *testing.T, ctx context.Context, runtime *Runtime, name string, resource core.PersistentResource, tool string, guest func(string, string) string) {
	t.Helper()
	kind := "nerdctl"
	if strings.Contains(tool, "docker") {
		kind = "docker"
	}
	f := &runtimeImageFixture{runtime: runtime, name: name, resource: resource, instance: "env-" + strings.Repeat("d", 32)}
	service := &oci.ManagedImages{Catalog: f, Environments: f}
	all, err := service.List(ctx, name, kind)
	if err != nil {
		t.Fatal("managed runtime image list", kind, err)
	}
	if len(all.Images) != 1 {
		t.Fatal("unexpected isolated runtime image inventory", kind, len(all.Images))
	}
	id := all.Images[0].ID
	// A stopped container must also keep its image in use.
	guest(name, tool+" create --name image-guard --pull never --network none hacocoon-area:local")
	if err := service.Delete(ctx, all.Target, id); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal("stopped container image not protected", kind, err)
	}
	guest(name, tool+" rm image-guard")
	if err := service.Delete(ctx, all.Target, id); err != nil {
		t.Fatal("managed runtime image delete", kind, err)
	}
	after, err := service.List(ctx, name, kind)
	if err != nil || len(after.Images) != 0 {
		t.Fatal("runtime image absence unproven", kind, err)
	}
	t.Log("PASS managed", kind, "image inventory, stopped-container refusal, immutable runtime ID deletion and absence; source-copy independence checked by caller")
}
