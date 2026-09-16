//go:build linux

package incus

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/base/manage"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
)

func TestRealIncusBaseArchiveImportE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_BASE_BUILD") != "1" {
		t.Skip("set HACO_E2E_BASE_BUILD=1 on a dedicated Incus host")
	}
	pool, image, binary := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_INCUS_RESUME_IMAGE"), os.Getenv("HACO_E2E_SNAPSHOT_CLI")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || !baseFingerprintPattern.MatchString(image) || binary == "" {
		t.Fatal("explicit root/pool/image/CLI required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	dir, err := os.MkdirTemp("/var/lib", "haco-base-import-")
	must(err)
	t.Logf("owned fixture catalog=%s; failures retain exact ownership evidence", filepath.Join(dir, "state.json"))
	r := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
	r.setRootPool(pool)
	p, err := NewSandboxProvider(r)
	must(err)
	p.sources["fixture-parent"] = "local:" + image
	store := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json"))
	envs := workspace.New(p, store)
	work := core.NewTemporaryWorkspace()
	name := "import-source-" + strings.TrimPrefix(filepath.Base(dir), "haco-base-import-")
	source, err := envs.Create(ctx, core.EnvironmentSpec{Name: name, Base: "fixture-parent", TemporaryWorkspace: &work, SkipDefaultResource: true})
	must(err)
	executed, err := envs.ExecForWorkspace(ctx, name, work.ID, core.ExecutionRequest{Argv: []string{"/bin/sh", "-eu", "-c", "printf '#!/bin/sh\necho imported-tool\n' > /usr/local/bin/imported-tool; chmod 0755 /usr/local/bin/imported-tool"}})
	must(err)
	if executed.ExitCode != 0 {
		t.Fatal("prepare source tool")
	}
	must(envs.StopForWorkspace(ctx, name, work.ID))
	lease, err := store.GetWorkspaceLease(ctx, name)
	must(err)
	plan := snapshotRootfsPlan{Pool: pool, Source: source.RuntimeRef, SourceInstanceID: lease.InstanceID, Owner: strings.TrimPrefix(lease.InstanceID, "env-")}
	component, err := r.snapshotComponent(snapshotBinding{Version: 1, Project: r.project, Rootfs: &plan})
	must(err)
	receipt, err := os.OpenFile(filepath.Join(dir, "saved-rootfs.json"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	must(json.NewEncoder(receipt).Encode(component))
	must(receipt.Sync())
	must(receipt.Close())
	must(r.createSnapshotRootfs(ctx, plan))
	must(r.verifySnapshotRootfs(ctx, plan))
	component.State = "verified"
	// This private test source has a single owner and no concurrent deletion path.
	archive, err := r.ExportSnapshotRootfs(ctx, component, dir, basebuild.MaxArchiveBytes)
	must(err)
	file := filepath.Join(dir, "input.tar")
	output, err := os.OpenFile(file, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	_, err = io.Copy(output, archive.Reader())
	must(err)
	must(output.Sync())
	must(output.Close())
	must(archive.Close())
	must(envs.DeleteTemporary(ctx, name, work))
	must(r.deleteSnapshotRootfs(ctx, plan))
	service := &basebuild.Service{Environments: envs}
	server := control.NewServer()
	must(controlapi.RegisterBaseImport(server, func(ctx context.Context, input io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
		return service.Import(ctx, req, input, dir)
	}))
	socket := filepath.Join(dir, "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	must(err)
	serving, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- server.Serve(serving, listener) }()
	defer func() { stop(); <-done }()
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	base := core.BaseName("imported-" + strings.TrimPrefix(filepath.Base(dir), "haco-base-import-"))
	command := exec.CommandContext(ctx, binary, "base", "import", "--name", string(base), "--json", file)
	var stderr strings.Builder
	command.Stderr = &stderr
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("public Base import failed: %v; %s; %s", err, stderr.String(), raw)
	}
	var result basebuild.Result
	must(json.Unmarshal(raw, &result))
	if result.State != "ready" || result.Builder != "" || result.Base.Name != base || result.Base.Revision == "" {
		t.Fatal("incomplete import", result)
	}
	logical, err := p.InspectBase(ctx, base)
	must(err)
	if logical.Revision != result.Base.Revision {
		t.Fatal("mutable or unconfirmed Base")
	}
	nextWork := core.NewTemporaryWorkspace()
	next, err := envs.Create(ctx, core.EnvironmentSpec{Name: string(base), Base: base, TemporaryWorkspace: &nextWork, SkipDefaultResource: true})
	must(err)
	read, err := envs.ExecForWorkspace(ctx, next.Name, nextWork.ID, core.ExecutionRequest{Argv: []string{"/usr/local/bin/imported-tool"}})
	must(err)
	if read.ExitCode != 0 || strings.TrimSpace(read.Stdout) != "imported-tool" {
		t.Fatal("imported tool unusable")
	}
	must(envs.DeleteTemporary(ctx, next.Name, nextWork))
	manager := &basemanage.Service{Backend: p.BaseProvider, Catalog: store}
	images, err := manager.List(ctx)
	must(err)
	selected := false
	for _, owned := range images {
		if owned.Name == base && "sha256:"+owned.Fingerprint == string(result.Base.Revision) {
			if selected {
				t.Fatal("duplicate image identity")
			}
			selected = true
			must(manager.Delete(ctx, owned.Identity))
		}
	}
	if !selected {
		t.Fatal("imported Base missing before cleanup")
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatal("source archive deleted", err)
	}
	t.Logf("PASS actual CLI archive import, isolated temporary builder, immutable Base %s, fresh Env tool use, retained input and exact-owned cleanup; not performance or authenticated acceptance", result.Base.Revision)
}
