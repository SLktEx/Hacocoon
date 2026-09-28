//go:build linux

package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	basebuild "github.com/SLktEx/Hacocoon/internal/base/build"
	basemanage "github.com/SLktEx/Hacocoon/internal/base/manage"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/state"
	ociplugin "github.com/SLktEx/Hacocoon/internal/storage/oci"
	persistentresource "github.com/SLktEx/Hacocoon/internal/storage/resource"
	"github.com/SLktEx/Hacocoon/internal/workspace"
)

// The maintained CI invokes this exact test through ci_required_tests.py, so a
// skipped prerequisite or a missing test cannot be reported as acceptance.
func TestRealIncusPackerBaseBuildE2E(t *testing.T) {
	if os.Getenv("HACO_E2E_PACKER") != "1" {
		t.Skip("set HACO_E2E_PACKER=1 with Incus and candidate companions")
	}
	binary := os.Getenv("HACO_E2E_SNAPSHOT_CLI")
	if os.Geteuid() != 0 || binary == "" {
		t.Fatal("root and candidate CLI/companions required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
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
		result, err := runner.Run(ctx, "incus", args...)
		if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
			t.Fatalf("Incus fixture command failed: %v: %v %s", args, err, result.Stderr)
		}
		return result.Stdout
	}
	var nonce [8]byte
	_, err := rand.Read(nonce[:])
	must(err)
	project := "haco-packer-e2e-" + hex.EncodeToString(nonce[:])
	pool := project
	t.Logf("exact fixture project/pool=%s; failed tests retain resources for diagnosis", project)
	storage := "size=16GiB"
	if source := os.Getenv("HACO_E2E_PACKER_STORAGE_SOURCE"); source != "" {
		if !filepath.IsAbs(source) {
			t.Fatal("fixture Btrfs source must be absolute")
		}
		storage = "source=" + filepath.Join(source, project)
	}
	command("storage", "create", pool, "btrfs", storage, "user.hacocoon.fixture="+project)
	r := New(WrapEnvironmentNetworkOwnershipRunner(runner))
	r.project = project
	r.setRootPool(pool)
	directory, err := os.MkdirTemp("/var/lib", "haco-packer-e2e-")
	must(err)
	store := state.NewEnvironmentJSONStore(filepath.Join(directory, "state.json"))
	backend := &PersistentResourceBackend{Runtime: r}
	resources := &persistentresource.Service{Store: store, Backend: backend}
	stores := ociplugin.WorkspaceStores{Resources: resources}
	r.ConfigureHostStorage(func(ctx context.Context) error {
		if err := stores.EnsureHost(ctx, backend); err != nil {
			return err
		}
		source, err := store.GetPersistentResource(ctx, ociplugin.HostStoreID)
		if err != nil {
			return err
		}
		if err := backend.EnableHostOCI(ctx, source); err != nil {
			return err
		}
		return backend.ProvisionHostTools(ctx, source)
	})
	must(r.SetupTrustedHost(ctx, filepath.Dir(binary)))
	t.Log("PASS fresh normal owned Host setup, Packer tooling and nested Incus initialization")
	guest := func(args ...string) string {
		t.Helper()
		argv := []string{"exec", trustedHostName, "--project", project, "--"}
		return command(append(argv, args...)...)
	}
	guest("/usr/local/bin/packer", "version")
	guest("/usr/bin/incus", "query", "/1.0")
	// Product setup may have an installed controller endpoint. An independent
	// fixture endpoint keeps test Bases out of that user's authoritative catalog.
	provider, err := NewSandboxProvider(r)
	must(err)
	envs := workspace.New(provider, store)
	service := &basebuild.Service{Environments: envs}
	server := control.NewServer()
	must(controlapi.RegisterBaseImport(server, func(ctx context.Context, input io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
		return service.Import(ctx, req, input, directory)
	}))
	socket := filepath.Join(directory, "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	must(err)
	serving, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- server.Serve(serving, listener) }()
	defer func() { stop(); <-done }()
	command("config", "device", "add", trustedHostName, "packer-fixture-control", "proxy", "bind=instance", "listen=unix:/var/lib/packer-fixture.sock", "connect=unix:"+socket, "mode=0600", "uid=0", "gid=0", "--project", project)
	guest("mkdir", "-m", "0700", "/root/packer-example")
	contextDirectory := os.Getenv("HACO_E2E_PACKER_CONTEXT")
	if contextDirectory == "" {
		contextDirectory = filepath.Join("..", "..", "..", "examples", "packer")
	}
	for _, name := range []string{"base.pkr.hcl", "setup.sh"} {
		source, err := filepath.Abs(filepath.Join(contextDirectory, name))
		must(err)
		command("file", "push", source, trustedHostName+"/root/packer-example/"+name, "--project", project, "--mode", "0644")
	}
	base := core.BaseName("packer-tools")
	build := func() basebuild.Result {
		t.Helper()
		out, runErr := runner.Run(ctx, "incus", "exec", trustedHostName, "--project", project, "--",
			"/usr/bin/env", "HACO_CONTROL_SOCKET=/var/lib/packer-fixture.sock", "haco", "base", "build", "--name", string(base), "--json", "--output", "/root/packer-example")
		var result basebuild.Result
		must(json.Unmarshal([]byte(out.Stdout), &result))
		if runErr != nil || out.ExitCode != 0 {
			// Never log subprocess text. Emit only fixed allowlisted categories.
			t.Fatalf("Packer fixture failed; categories=%v", packerFailureCategories(result.Execution))
		}
		if result.State != "ready" || result.Base.Name != base || result.Base.Revision == "" || result.Builder != "" {
			t.Fatal("incomplete Packer result", result)
		}
		return result
	}
	first := build()
	t.Log("PASS real packer init/plugin download/load/validate/build, shell script, nested image/export, controller stream/import/publication")
	work := core.NewTemporaryWorkspace()
	env, err := envs.Create(ctx, core.EnvironmentSpec{Name: "packer-verify", Base: base, TemporaryWorkspace: &work, SkipDefaultResource: true})
	must(err)
	if env.Base == nil || env.Base.Revision != first.Base.Revision {
		t.Fatal("first Env did not pin the published revision")
	}
	runTool := func() string {
		t.Helper()
		result, err := envs.ExecForWorkspace(ctx, env.Name, work.ID, core.ExecutionRequest{Argv: []string{"/usr/local/bin/my-tool"}})
		must(err)
		if result.ExitCode != 0 {
			t.Fatal("tool exit", result.ExitCode)
		}
		return strings.TrimSpace(result.Stdout)
	}
	if runTool() != "hello-from-packer" {
		t.Fatal("provisioner result missing")
	}
	clean := func() {
		t.Helper()
		output := guest("/usr/bin/python3", "-I", "-c", `import json, pathlib, subprocess
root=pathlib.Path("/var/lib/hacocoon-packer/builds")
assert list(root.iterdir()) == []
projects=json.loads(subprocess.check_output(["incus","query","/1.0/projects?recursion=1"]))
assert not any(p["name"].startswith("haco-packer-") for p in projects)
print("clean")`)
		if strings.TrimSpace(output) != "clean" {
			t.Fatal("nested resources remain")
		}
	}
	clean()
	// Rebuild a different rootfs to prove immutable provenance and pointer move.
	guest("/bin/sh", "-ec", `printf '%s\n' '#!/bin/sh' 'set -eu' 'printf "#!/bin/sh\\necho hello-from-packer-two\\n" > /usr/local/bin/my-tool' 'chmod 0755 /usr/local/bin/my-tool' > /root/packer-example/setup.sh`)
	second := build()
	if second.Base.Revision == first.Base.Revision || runTool() != "hello-from-packer" {
		t.Fatal("rebuild mutated existing revision")
	}
	nextWork := core.NewTemporaryWorkspace()
	next, err := envs.Create(ctx, core.EnvironmentSpec{Name: "packer-verify-next", Base: base, TemporaryWorkspace: &nextWork, SkipDefaultResource: true})
	must(err)
	retained, err := store.GetEnvironment(ctx, env.Name)
	must(err)
	if retained.Base == nil || retained.Base.Revision != first.Base.Revision || next.Base == nil || next.Base.Revision != second.Base.Revision {
		t.Fatal("immutable Base provenance changed")
	}
	got, err := envs.ExecForWorkspace(ctx, next.Name, nextWork.ID, core.ExecutionRequest{Argv: []string{"/usr/local/bin/my-tool"}})
	must(err)
	if got.ExitCode != 0 || strings.TrimSpace(got.Stdout) != "hello-from-packer-two" {
		t.Fatal("new Env did not resolve new revision")
	}
	clean()
	manager := &basemanage.Service{Backend: provider.BaseProvider, Catalog: store}
	originalHCL, err := filepath.Abs(filepath.Join(contextDirectory, "base.pkr.hcl"))
	must(err)
	for _, failure := range []struct{ name, script, stage string }{
		{"invalid-hcl", `printf '%s\n' 'invalid {' >> /root/packer-example/base.pkr.hcl`, "fmt"},
		{"init", `sed -i 's/= 1.0.5/= 9999.0.0/' /root/packer-example/base.pkr.hcl`, "init"},
		{"provisioner", `printf '%s\n' '#!/bin/sh' 'exit 73' > /root/packer-example/setup.sh`, "build"},
	} {
		command("file", "push", originalHCL, trustedHostName+"/root/packer-example/base.pkr.hcl", "--project", project, "--mode", "0644")
		guest("/bin/sh", "-ec", failure.script)
		out, err := runner.Run(ctx, "incus", "exec", trustedHostName, "--project", project, "--",
			"/usr/bin/env", "HACO_CONTROL_SOCKET=/var/lib/packer-fixture.sock", "haco", "base", "build", "--name", string(base), "--json", "/root/packer-example")
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 1 || out.ExitCode != 1 || out.StdoutTruncated || ctx.Err() != nil {
			t.Fatalf("%s did not return the expected build failure: exit=%d error=%v", failure.name, out.ExitCode, err)
		}
		var failed basebuild.Result
		must(json.Unmarshal([]byte(out.Stdout), &failed))
		if out.ExitCode == 0 || failed.State == "ready" || failed.Stage != failure.stage || !strings.HasPrefix(failed.Builder, "haco-packer-") {
			t.Fatalf("%s failure was not observed at expected stage: %+v", failure.name, failed)
		}
		images, err := manager.List(ctx)
		must(err)
		current := 0
		for _, image := range images {
			if image.Name == base && image.Current {
				current++
				if core.BaseRevision("sha256:"+image.Fingerprint) != second.Base.Revision {
					t.Fatal("failed build changed Base pointer")
				}
			}
		}
		if current != 1 || runTool() != "hello-from-packer" {
			t.Fatal("failed build damaged published Bases")
		}
		t.Logf("PASS real %s failure preserves Base pointer and original Env; retained receipt %s", failure.name, failed.Builder)
	}
	must(envs.DeleteTemporary(ctx, env.Name, work))
	must(envs.DeleteTemporary(ctx, next.Name, nextWork))
	images, err := manager.List(ctx)
	must(err)
	for _, image := range images {
		if image.Name == base {
			must(manager.Delete(ctx, image.Identity))
		}
	}
	t.Log("PASS ordinary Env my-tool=hello-from-packer; rebuild preserves old Env; new Env uses new immutable revision; no nested temporary instance/image/artifact/context")
	// Keep the exactly named outer fixture for provider diagnostics; it contains
	// no user data. Successful-build resources and imported Bases are gone;
	// failed-attempt receipts and their exact nested resources remain diagnosable.
}

// External error text is untrusted even in synthetic E2E. Only these fixed
// categories may cross into CI logs; no matching substring or value is emitted.
func packerFailureCategories(execution *core.ExecutionResult) []string {
	if execution == nil {
		return []string{"no-execution-result"}
	}
	text := strings.ToLower(execution.Stdout + execution.Stderr)
	var categories []string
	for _, item := range []struct{ needle, category string }{
		{"error creating container", "instance-create"}, {"error publishing container", "image-publish"},
		{"error stopping container", "instance-stop"}, {"error uploading", "upload"},
		{"error executing", "provisioner-exec"}, {"idmap", "idmap"}, {"uid_map", "uid-map"},
		{"uid/gid", "uid-gid"}, {"apparmor", "apparmor"}, {"operation not permitted", "operation-not-permitted"},
		{"permission denied", "permission-denied"}, {"no space left", "disk-full"},
		{"failed to mount", "mount"}, {"failed to start", "start"}, {"failed to run", "run"},
		{"no root device", "root-device"}, {"failed to connect", "connect"},
		{"no such file", "missing-file"}, {"not found", "not-found"},
		{"network", "network"}, {"dhcp", "dhcp"}, {"timeout", "timeout"},
		{"timed out", "timed-out"}, {"unknown configuration", "unknown-config"},
		{"not supported", "unsupported"}, {"failed to set", "set-config"},
		{"subuid", "subuid"}, {"subgid", "subgid"}, {"exit status", "child-exit"},
		{"x509", "tls-certificate"}, {"connection refused", "connection-refused"},
		{"device", "device"}, {"resource temporarily unavailable", "resource-unavailable"},
	} {
		if strings.Contains(text, item.needle) {
			categories = append(categories, item.category)
		}
	}
	if len(categories) == 0 {
		return []string{"unclassified"}
	}
	return categories
}
func TestPackerFailureCategoriesNeverExposeSubprocessText(t *testing.T) {
	got := packerFailureCategories(&core.ExecutionResult{Stdout: "secret-token\x1b[2J Error creating container", Stderr: "Permission denied /private/user-data"})
	if strings.Join(got, ",") != "instance-create,permission-denied" {
		t.Fatal("unexpected diagnostic categories")
	}
	if strings.Join(packerFailureCategories(&core.ExecutionResult{Stderr: "private unmatched text"}), ",") != "unclassified" {
		t.Fatal("unrecognized text exposed")
	}
}
