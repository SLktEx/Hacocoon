//go:build linux

package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

// Use the shipped composition with an empty catalog. The aggregate's catalog is
// never adopted; only the immutable bundle crosses this controller boundary.
func verifyImportControllerCLI(t *testing.T, ctx context.Context, runtime *Runtime, directory, bundle, name string, oldIDs []string) {
	t.Helper()
	controller, product := os.Getenv("HACO_E2E_IMPORT_CONTROLLER"), os.Getenv("HACO_E2E_SNAPSHOT_CLI")
	if controller == "" {
		t.Log("SKIP shipped import controller: explicit binary not supplied")
		return
	}
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("HACO_CI_RUNNER_ENVIRONMENT") != "github-hosted" || !filepath.IsAbs(controller) || !filepath.IsAbs(product) {
		t.Fatal("import controller fixture requires explicit binaries and disposable GitHub-hosted runner")
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(directory, "import-controller")
	must(os.Mkdir(root, 0700))
	temporary := filepath.Join(root, "tmp")
	must(os.Mkdir(temporary, 0700))
	socket := filepath.Join(root, "control.sock")
	environment := append(os.Environ(), "HACO_ROOT="+root, "TMPDIR="+temporary, "HACO_CONTROL_SOCKET="+socket, "HACO_PLUGIN_OCI=", "HACO_RUNTIME_PROVIDER="+environmentapp.ProviderIncus)
	log, err := os.OpenFile(filepath.Join(root, "controller.log"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	must(err)
	defer log.Close()
	process := exec.CommandContext(ctx, controller)
	process.Env, process.Stdout, process.Stderr = environment, log, log
	must(process.Start())
	done := make(chan error, 1)
	go func() { done <- process.Wait() }()
	stopped := false
	stop := func() {
		if stopped {
			return
		}
		stopped = true
		_ = process.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(30 * time.Second):
			_ = process.Process.Kill()
			<-done
			t.Error("import controller exceeded shutdown bound")
		}
	}
	defer stop()
	deadline := time.Now().Add(30 * time.Second)
	for {
		info, err := os.Lstat(socket)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			break
		}
		select {
		case <-done:
			stopped = true
			t.Fatalf("import controller exited before readiness; private diagnostics: %s", log.Name())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("import controller socket not ready; private diagnostics: %s", log.Name())
		}
		time.Sleep(50 * time.Millisecond)
	}
	invoke := func(args ...string) []byte {
		t.Helper()
		command := exec.CommandContext(ctx, product, args...)
		var output bytes.Buffer
		command.Env, command.Stdout, command.Stderr = environment, &output, log
		if err := command.Run(); err != nil || output.Len() > 1<<20 {
			t.Fatalf("import controller CLI operation=%s failed; private diagnostics: %s", args[1], log.Name())
		}
		return output.Bytes()
	}
	var receipt environmenttransfer.ImportResult
	must(json.Unmarshal(invoke("env", "import", "--json", bundle, name), &receipt))
	if receipt.Environment != name || receipt.State != "running" {
		t.Fatal("shipped controller did not report running destination")
	}
	catalog := state.NewEnvironmentJSONStore(filepath.Join(root, "state", "environments.json"))
	imported, err := catalog.GetEnvironment(ctx, name)
	must(err)
	native := "haco-" + name
	if !environmentapp.MatchesRuntimeRef(imported.RuntimeRef, environmentapp.ProviderIncus, native) {
		t.Fatal("imported runtime route does not match native identity")
	}
	identity, err := catalog.EnvironmentInstance(ctx, imported)
	must(err)
	if imported.Base != nil {
		t.Fatal("imported Env depends on Base")
	}
	must(runtime.VerifyEnvironmentIdentity(ctx, native, identity))
	for _, old := range oldIDs {
		if identity == old || runtime.VerifyEnvironmentIdentity(ctx, native, old) == nil {
			t.Fatal("import accepted source generation")
		}
	}
	repository := &RepositoryBackend{Runtime: runtime}
	repositories := gitrepo.NewRepositoryService(filepath.Join(root, "state", "repositories"), repository)
	repositories.SnapshotCatalog = catalog
	work, err := repositories.Get("work", receipt.Workspace)
	must(err)
	store, err := catalog.GetPersistentResource(ctx, receipt.OCI)
	must(err)
	if store.WorkspaceID != imported.Workspace.ID || imported.Workspace.Path != "managed:"+work.ID {
		t.Fatal("imported Store lost Workspace association")
	}
	mounts, err := repository.WorkspaceAttachments(ctx, work)
	must(err)
	if len(mounts) != 2 {
		t.Fatal("imported collection incomplete")
	}
	readGuest := func(path, expected string) {
		t.Helper()
		result, err := runtime.runner.Run(ctx, "incus", "exec", native, "--project", runtime.project, "--", "cat", path)
		if err != nil || result.ExitCode != 0 || result.StdoutTruncated || strings.TrimSpace(result.Stdout) != expected {
			t.Fatal("shipped controller imported data mismatch")
		}
	}
	readGuest("/root/snapshot-marker", "guest-only bytes")
	readGuest("/root/.ssh/authorized_keys", "ssh-ed25519 AAAA user-key")
	for _, mount := range mounts {
		readGuest(mount.Path+"/tracked", "uncommitted "+mount.Device)
		readGuest(mount.Path+"/untracked", "untracked "+mount.Device)
	}
	readGuest(OCIStorePath+"/containerd/data", "actual stored bytes")
	invoke("env", "delete", name)
	if exists, err := runtime.environmentExists(ctx, native); err != nil || exists {
		t.Fatal("deleted imported instance absence unproven")
	}
	persistent := &PersistentResourceBackend{Runtime: runtime}
	must(persistent.Verify(ctx, store))
	_, err = repository.WorkspaceAttachments(ctx, work)
	must(err)
	file, err := os.Open(bundle)
	must(err)
	_, inspectErr := environmenttransfer.Inspect(io.NewSectionReader(file, 0, 1<<63-1), 4<<30)
	must(file.Close())
	must(inspectErr)
	// Stop all controller operations before test-only explicit owned-data cleanup.
	// Canonical lease and owner checks remain in force; failures retain this catalog.
	stop()
	t.Setenv("TMPDIR", temporary)
	cleanup := workspace.NewWithProvider(nil, catalog, aggregateWorkspaceResolver{repositories})
	stores := persistentresource.Service{Store: catalog, Backend: persistent}
	must(cleanup.CleanupRestoredData(ctx, imported.Workspace, func(ctx context.Context) error {
		if err := stores.DeleteRestoredCopy(ctx, store); err != nil {
			return err
		}
		return repositories.DeleteWorkspace(ctx, work.ID, work.Owner)
	}))
	t.Log("PASS shipped import controller/CLI: empty catalog, real native data and running Env, fresh generation, no Base, managed SSH reset, retained Workspace/OCI after Env deletion and canonical owned cleanup; installed desktop, SSH handshake and live OCI remain unverified")
}
