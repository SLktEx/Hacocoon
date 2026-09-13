package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRealIncusSourceDeletionE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_WORKSPACE_DELETE") != "1" {
		t.Skip("set HACO_E2E_WORKSPACE_DELETE=1 on dedicated Incus/Btrfs host")
	}
	pool, binary := os.Getenv("HACO_E2E_INCUS_RESUME_POOL"), os.Getenv("HACO_E2E_SNAPSHOT_CLI")
	if os.Geteuid() != 0 || !safeIncusRef(pool) || binary == "" {
		t.Fatal("root, explicit pool and built product CLI required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	runner := host.ExecRunner{}
	command := func(args ...string) string {
		t.Helper()
		out, err := runner.Run(ctx, "incus", args...)
		if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
			t.Fatal("native command failed", args)
		}
		return out.Stdout
	}
	var nonce [16]byte
	_, err := rand.Read(nonce[:])
	must(err)
	owner := hex.EncodeToString(nonce[:])
	project := "haco-source-" + owner[:16]
	r := New(runner)
	r.project = project
	r.setRootPool(pool)
	o := gitrepo.Object{Kind: "repo", ID: "source", Repository: "source", Remote: "https://github.com/example/source.git", Branch: "main", NativeRef: pool + "/haco-repo-source", Owner: owner, State: "ready"}
	dir, err := os.MkdirTemp("/var/lib", "haco-source-delete-")
	must(err)
	write := func(name string, v any) {
		t.Helper()
		raw, err := json.Marshal(v)
		must(err)
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		must(err)
		_, err = f.Write(raw)
		must(err)
		must(f.Sync())
		must(f.Close())
	}
	write("fixture.json", map[string]any{"project": project, "pool": pool, "host": trustedHostName, "source": o})
	write("repo-source.json", o)
	t.Log("exact isolated fixture receipt", dir, project)
	command("project", "create", project, "--config", "features.images=false", "--config", "features.profiles=false")
	// This fixture only inspects a stopped Host's managed volume attachment.
	// An empty native instance avoids unrelated image/project dependencies.
	command("init", "--empty", trustedHostName, "--project", project, "--storage", pool, "--no-profiles", "--config", trustedHostRoleKey+"="+trustedHostRoleValue)
	backend := &RepositoryBackend{Runtime: r}
	must(backend.CreateVolume(ctx, o, nil))
	command("config", "device", "add", trustedHostName, "haco-repo-source", "disk", "pool="+pool, "source=haco-repo-source", "path="+gitrepo.RepositoryRoot+"/source", "--project", project)
	service := gitrepo.NewRepositoryService(dir, backend)
	server := control.NewServer()
	must(controlapi.RegisterRepositories(server, service, nil))
	socket := filepath.Join(dir, "cli.sock")
	listener, err := control.ListenUnix(socket, 0600)
	must(err)
	serveCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- server.Serve(serveCtx, listener) }()
	defer func() { stop(); <-done }()
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	cli := func(args ...string) ([]byte, error) {
		return exec.CommandContext(ctx, binary, args...).CombinedOutput()
	}
	output, err := cli("repo", "list", "--json")
	must(err)
	var listed controlapi.RepositoryManageResponse
	must(json.Unmarshal(output, &listed))
	if len(listed.Sources) != 1 || listed.Sources[0].Source.Owner != owner {
		t.Fatal("CLI source review lost identity")
	}
	work := o
	work.Kind = "work"
	work.ID = "reserved"
	work.NativeRef = pool + "/haco-work-reserved"
	work.State = "creating"
	write("work-reserved.json", work)
	if output, err := cli("repo", "delete", "--yes", "source"); err == nil || !strings.Contains(string(output), "referenced") {
		t.Fatalf("Workspace reference refusal: %v %s", err, output)
	}
	must(os.Remove(filepath.Join(dir, "work-reserved.json")))
	command("storage", "volume", "snapshot", "create", pool, "haco-repo-source", "keep", "--project", project)
	if err := service.DeleteSource(ctx, o.ID, o.Owner); !errors.Is(err, core.ErrStorageBusy) {
		t.Fatal("saved child refusal", err)
	}
	if got, err := service.Get("repo", o.ID); err != nil || got.State != "ready" {
		t.Fatal("refusal changed catalog", got, err)
	}
	if mounted, err := backend.sourceDevice(ctx, o); err != nil || !mounted {
		t.Fatal("refusal detached Host", err)
	}
	command("storage", "volume", "snapshot", "show", pool, "haco-repo-source", "keep", "--project", project)
	command("storage", "volume", "snapshot", "delete", pool, "haco-repo-source", "keep", "--project", project)
	if err := service.DeleteSource(ctx, o.ID, strings.Repeat("f", 32)); !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatal("stale owner accepted", err)
	}
	output, err = cli("repo", "delete", "--yes", "source")
	if err != nil {
		t.Fatalf("public delete: %v %s", err, output)
	}
	if mounted, err := backend.sourceDevice(ctx, o); err != nil || mounted {
		t.Fatal("Host mount remains", err)
	}
	if v, err := backend.managedVolumeForDeletion(ctx, o, ""); err != nil || v != nil {
		t.Fatal("native absence unproven", err)
	}
	if _, err := service.Get("repo", o.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatal("catalog remains", err)
	}
	must(r.verifyTrustedHostOwnership(ctx))
	command("delete", trustedHostName, "--project", project)
	command("project", "delete", project)
	t.Log("PASS public source list/delete, Workspace reservation refusal, native child and Host mount retention, stale owner refusal, exact mount removal/native absence; shared image/pool retained")
}
