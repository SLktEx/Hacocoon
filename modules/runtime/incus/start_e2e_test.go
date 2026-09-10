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
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
)

// This fixture tests resume on an independent instance in an existing managed
// project/pool. It never starts, mounts or deletes another Environment.
func TestRealIncusResumeE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_INCUS_RESUME") != "1" {
		t.Skip("set HACO_E2E_INCUS_RESUME=1 with cached image/pool on a supported root Incus host")
	}
	image, pool := os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE"), os.Getenv("HACO_E2E_INCUS_RESUME_POOL")
	if os.Geteuid() != 0 || !baseFingerprintPattern.MatchString(image) || pool == "" || strings.HasPrefix(pool, "-") {
		t.Fatal("root, full cached image fingerprint and explicit pool required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	r := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
	r.setRootPool(pool)
	p, err := NewSandboxProvider(r)
	if err != nil {
		t.Fatal(err)
	}
	var nonce [8]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	name := "resume-e2e-" + hex.EncodeToString(nonce[:])
	ref := "haco-" + name
	// Retain the ownership ledger even if native cleanup is uncertain.
	root, err := os.MkdirTemp("", "haco-resume-e2e-")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("retained native ownership ledger: %s", root)
	work := filepath.Join(root, "work")
	if err := os.Mkdir(work, 0755); err != nil {
		t.Fatal(err)
	}
	// The provider fixture starts stopped; use canonical transitions for ownership.
	st := state.NewEnvironmentJSONStore(filepath.Join(root, "state.json"))
	instanceID, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	lease := core.WorkspaceLease{InstanceID: instanceID, EnvironmentID: name, WorkspaceID: core.WorkspaceID(work), SourcePath: work, AccessMode: core.WorkspaceReadWrite, Owner: name, State: core.WorkspaceLeaseAcquiring, AcquiredAt: time.Now()}
	if err := st.BeginEnvironmentCreate(ctx, lease); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		result, err := r.runner.Run(ctx, "incus", args...)
		if err != nil {
			t.Fatalf("%v: %v %s", args, err, result.Stderr)
		}
		return result.Stdout
	}
	run("init", "local:"+image, ref, "--project", r.project, "--no-profiles", "--storage", pool, "--config", managedEnvironmentMarkerKey+"="+managedEnvironmentMarkerValue, "--config", environmentInstanceKey+"="+instanceID)
	lease.RuntimeRef = ref
	if err := st.RecordEnvironmentRuntime(ctx, lease); err != nil {
		t.Fatalf("ownership recording failed; retain %s: %v", ref, err)
	}
	svc := workspace.New(p, st)
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if err := svc.Delete(cleanup, name); err != nil {
			t.Errorf("cleanup retained %s: %v", ref, err)
		}
	}()
	t.Logf("test-owned runtime: %s", ref)
	if err := p.VerifyEnvironmentIdentity(ctx, ref, instanceID); err != nil {
		t.Fatal(err)
	}
	otherID, err := core.NewEnvironmentInstanceID()
	if err != nil {
		t.Fatal(err)
	}
	if err := p.VerifyEnvironmentIdentity(ctx, ref, otherID); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatalf("foreign creation ID accepted: %v", err)
	}
	if err := r.ensureRoutedSandboxHost(ctx); err != nil {
		t.Fatal(err)
	}
	if err := r.addRoutedSandboxNIC(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if err := p.addWorkspaceDevice(ctx, ref, core.EnvironmentRuntimeSpec{WorkspacePath: work}); err != nil {
		t.Fatal(err)
	}
	lease.State = core.WorkspaceLeaseActive
	env := core.Environment{Name: name, RuntimeRef: ref, Workspace: core.Workspace{ID: lease.WorkspaceID, Path: work}, AccessMode: lease.AccessMode, CreatedAt: time.Now()}
	if err := st.CommitEnvironmentCreate(ctx, env, lease); err != nil {
		t.Fatal(err)
	}
	lease, err = st.GetWorkspaceLease(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Start(ctx, name); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(run("config", "get", ref, "boot.autostart", "--project", r.project)); got != "false" {
		t.Fatalf("legacy resume left Incus autostart enabled: %q", got)
	}
	run("exec", ref, "--project", r.project, "--", "sh", "-ceu", "printf retained-root > /root/resume-marker; printf retained-work > /workspace/resume-marker")
	if err := svc.Stop(ctx, name); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := svc.Start(ctx, name); err != nil {
			t.Fatal(err)
		}
	}
	if got := run("exec", ref, "--project", r.project, "--", "cat", "/root/resume-marker", "/workspace/resume-marker"); got != "retained-rootretained-work" {
		t.Fatalf("lost contents: %q", got)
	}
	after, err := st.GetWorkspaceLease(ctx, name)
	if err != nil || after != lease {
		t.Fatalf("lease changed: %+v %v", after, err)
	}
	if err := svc.Delete(ctx, name); err != nil {
		t.Fatal(err)
	}
	t.Log("PASS exact provider creation identity, foreign identity refusal, stop/start, repeated start, root and Workspace retention, unchanged lease and canonical cleanup")
}
