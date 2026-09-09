package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealIncusBaseBuildE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_BASE_BUILD") != "1" {
		t.Skip("set HACO_E2E_BASE_BUILD=1 on dedicated Incus/Btrfs host")
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
	dir, err := os.MkdirTemp("/var/lib", "haco-base-build-")
	must(err)
	t.Log("failure evidence directory", dir)
	t.Setenv("TMPDIR", t.TempDir())
	r := New(WrapEnvironmentNetworkOwnershipRunner(host.ExecRunner{}))
	r.setRootPool(pool)
	p, err := NewSandboxProvider(r)
	must(err)
	p.sources["fixture-parent"] = "local:" + image
	store := state.NewEnvironmentJSONStore(filepath.Join(dir, "state.json"))
	envs := workspace.New(p, store)
	service := &basebuild.Service{Environments: envs}
	server := control.NewServer()
	must(controlapi.RegisterBaseBuild(server, service))
	socket := filepath.Join(dir, "cli.sock")
	listener, err := control.ListenUnix(socket, 0600)
	must(err)
	serveCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- server.Serve(serveCtx, listener) }()
	defer func() { stop(); <-done }()
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	name := core.BaseName("test-tools-" + strings.TrimPrefix(filepath.Base(dir), "haco-base-build-"))
	var revisions []string
	build := func(version string) core.BaseInfo {
		t.Helper()
		definition := basebuild.Definition{Name: name, From: "fixture-parent", Run: "printf '#!/bin/sh\\necho " + version + "\\n' > /usr/local/bin/haco-test-tool\nchmod 0755 /usr/local/bin/haco-test-tool\n"}
		data, err := json.Marshal(definition)
		must(err)
		path := filepath.Join(dir, "base.json")
		must(os.WriteFile(path, data, 0600))
		command := exec.CommandContext(ctx, binary, "base", "build", path)
		var stderr strings.Builder
		command.Stderr = &stderr
		out, err := command.Output()
		if err != nil {
			t.Fatalf("build failed: %v: %s: %s", err, stderr.String(), out)
		}
		var result basebuild.Result
		must(json.Unmarshal(out, &result))
		if result.State != "ready" || result.Builder != "" || result.Base.Revision == "" {
			t.Fatal("incomplete build", result)
		}
		revisions = append(revisions, strings.TrimPrefix(string(result.Base.Revision), "sha256:"))
		t.Log("published", result.Base)
		return result.Base
	}
	first := build("one")
	work, err := core.NewTemporaryWorkspace()
	must(err)
	env, err := envs.Create(ctx, core.EnvironmentSpec{Name: string(name), Base: name, TemporaryWorkspace: &work, SkipDefaultResource: true})
	must(err)
	if env.Base == nil || env.Base.Revision != first.Revision {
		t.Fatal("creation not pinned", env.Base)
	}
	read := func(want string) {
		t.Helper()
		out, err := envs.ExecForWorkspace(ctx, env.Name, work.ID, core.ExecutionRequest{Argv: []string{"/usr/local/bin/haco-test-tool"}})
		must(err)
		if out.ExitCode != 0 || strings.TrimSpace(out.Stdout) != want {
			t.Fatal(out)
		}
	}
	read("one")
	second := build("two")
	if first.Revision == second.Revision {
		t.Fatal("revision did not change")
	}
	read("one")
	latest, err := p.InspectBase(ctx, name)
	must(err)
	if latest.Revision != second.Revision {
		t.Fatal("pointer did not advance")
	}
	must(envs.DeleteTemporary(ctx, env.Name, work))
	work2, err := core.NewTemporaryWorkspace()
	must(err)
	env2, err := envs.Create(ctx, core.EnvironmentSpec{Name: string(name), Base: name, TemporaryWorkspace: &work2, SkipDefaultResource: true})
	must(err)
	out, err := envs.ExecForWorkspace(ctx, env2.Name, work2.ID, core.ExecutionRequest{Argv: []string{"/usr/local/bin/haco-test-tool"}})
	must(err)
	if out.ExitCode != 0 || strings.TrimSpace(out.Stdout) != "two" || env2.Base.Revision != second.Revision {
		t.Fatal("new create not updated", out, env2.Base)
	}
	must(envs.DeleteTemporary(ctx, env2.Name, work2))
	// Delete only this fixture's fully observed owned images, never the shared parent.
	for _, fingerprint := range revisions {
		im, err := p.ownedBaseImage(ctx, name, baseAlias{Target: fingerprint, Description: builtBaseDescription, Type: "container"})
		must(err)
		if im.Fingerprint == image {
			t.Fatal("shared parent selected")
		}
		must(p.baseQuery(ctx, "DELETE", p.basePath("/"+fingerprint), nil, nil))
	}
	stop()
	<-done
	done <- nil
	var remaining []baseImage
	must(p.baseQuery(ctx, "GET", p.basePath("")+"&recursion=1", nil, &remaining))
	if remaining == nil {
		t.Fatal("image absence unconfirmed")
	}
	for _, im := range remaining {
		for _, fingerprint := range revisions {
			if im.Fingerprint == fingerprint {
				t.Fatal("owned image remains")
			}
		}
	}
	must(os.RemoveAll(dir))
	t.Log("PASS actual CLI definition/build/register, pinned creation, existing Env unchanged by rebuild, future create updated, exact image/Env cleanup; SSH handshake is separate")
}
