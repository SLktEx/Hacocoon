package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

// This gate uses the shipped controller composition, catalog, lifecycle and CLI.
// Only initial synthetic Store contents are supplied by the native fixture.
func verifyMaintenanceControllerCLI(t *testing.T, ctx context.Context, runtime *Runtime, resource core.PersistentResource, receiptDirectory string) bool {
	t.Helper()
	controller, product := os.Getenv("HACO_E2E_MAINTENANCE_CONTROLLER"), os.Getenv("HACO_E2E_MAINTENANCE_CLI")
	if controller == "" && product == "" {
		t.Log("SKIP full maintenance controller/CLI: explicit binaries not supplied")
		return false
	}
	if os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("HACO_CI_RUNNER_ENVIRONMENT") != "github-hosted" || !filepath.IsAbs(controller) || !filepath.IsAbs(product) {
		t.Fatal("controller fixture requires explicit binaries and disposable GitHub-hosted runner")
	}
	root := filepath.Join(receiptDirectory, "controller")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(root, 0700))
	must(os.Mkdir(filepath.Join(root, "state"), 0700))
	// Earlier workflow steps run as the ordinary runner. Their lifecycle locks
	// must not be adopted by this root-only, independently owned fixture.
	temporary := filepath.Join(root, "tmp")
	must(os.Mkdir(temporary, 0700))
	catalog := state.NewEnvironmentJSONStore(filepath.Join(root, "state", "environments.json"))
	creating := resource
	creating.State = "creating"
	must(catalog.BeginPersistentResourceCreate(ctx, creating))
	must(catalog.CommitPersistentResourceCreate(ctx, creating))
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
	defer func() {
		if !stopped {
			_ = process.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				_ = process.Process.Kill()
				<-done
				t.Error("controller did not stop within its shutdown bound")
			}
		}
	}()
	deadline := time.Now().Add(30 * time.Second)
	for {
		info, err := os.Lstat(socket)
		if err == nil && info.Mode()&os.ModeSocket != 0 {
			break
		}
		select {
		case <-done:
			stopped = true
			t.Fatalf("controller exited before readiness; private diagnostics: %s", log.Name())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("controller socket not ready; private diagnostics: %s", log.Name())
		}
		time.Sleep(50 * time.Millisecond)
	}
	invoke := func(input string, success bool, args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(ctx, product, args...)
		var diagnostic bytes.Buffer
		cmd.Env, cmd.Stdin, cmd.Stderr = environment, strings.NewReader(input), io.MultiWriter(log, &diagnostic)
		output, err := cmd.Output()
		if (err == nil) != success || len(output) > 1<<20 || (!success && !strings.Contains(diagnostic.String(), "image is referenced by a container; retained")) {
			exitCode := -1
			if cmd.ProcessState != nil {
				exitCode = cmd.ProcessState.ExitCode()
			}
			t.Fatalf("maintenance CLI operation=%s exit_code=%d expected_success=%t context_done=%t category=%s stages=%s; private diagnostics: %s", strings.Join(args[:4], " "), exitCode, success, ctx.Err() != nil, maintenanceDiagnosticCategory(diagnostic.String()), maintenanceDiagnosticStages(diagnostic.String()), log.Name())
		}
		return output
	}
	providerRuns := func() string {
		t.Helper()
		result, err := runtime.runner.Run(ctx, "incus", "list", "--project", runtime.project, "--format", "csv", "-c", "n")
		must(err)
		if result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated {
			t.Fatal("native cleanup observation incomplete")
		}
		var names []string
		for _, name := range strings.Split(result.Stdout, "\n") {
			if strings.HasPrefix(name, "haco-run-") {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		return strings.Join(names, "\n")
	}
	baselineRuns := providerRuns()
	clean := func() {
		t.Helper()
		environments, err := catalog.ListEnvironments(ctx)
		must(err)
		runs, err := catalog.ListEphemeralRuns(ctx)
		must(err)
		leases, err := catalog.ListWorkspaceLeases(ctx)
		must(err)
		if len(environments) != 0 || len(runs) != 0 || len(leases) != 0 || providerRuns() != baselineRuns {
			t.Fatal("maintenance catalog or run ownership remained")
		}
		current, err := catalog.GetPersistentResource(ctx, resource.ID)
		must(err)
		if current.Ref() != resource.Ref() || current.NativeRef != resource.NativeRef || current.State != "ready" {
			t.Fatal("retained Store identity changed")
		}
		must((&PersistentResourceBackend{Runtime: runtime}).Verify(ctx, current))
	}
	list := func() oci.ManagedImageList {
		t.Helper()
		var result oci.ManagedImageList
		must(json.Unmarshal(invoke("", true, "plugin", "oci", "image", "list", "--json", resource.ID), &result))
		if !result.Target.Detached || result.Target.Store != resource.Ref() {
			t.Fatal("CLI did not review exact detached Store")
		}
		clean()
		return result
	}
	before := list()
	used, unused := "", ""
	for _, image := range before.Images {
		if len(image.Containers) != 0 {
			used = image.ID
		} else {
			unused = image.ID
		}
	}
	if used == "" || unused == "" {
		t.Fatal("fixture lacks referenced and unused digests")
	}
	invoke("yes\n", false, "plugin", "oci", "image", "delete", resource.ID, used)
	clean()
	var candidates oci.ManagedImageList
	must(json.Unmarshal(invoke("", true, "plugin", "oci", "image", "list", "--unused", "--json", resource.ID), &candidates))
	if candidates.Target != before.Target || len(candidates.Images) == 0 {
		t.Fatal("candidate target or images lost")
	}
	selected := map[string]bool{}
	for _, image := range candidates.Images {
		if len(image.Containers) != 0 || image.ID == used {
			t.Fatal("referenced candidate selected")
		}
		selected[image.ID] = true
	}
	if !selected[unused] {
		t.Fatal("unused image omitted")
	}
	clean()
	invoke("no\n", false, "plugin", "oci", "image", "delete", "--unused", resource.ID)
	clean()
	retained := list()
	if len(retained.Images) != len(before.Images) {
		t.Fatal("declined candidates changed")
	}
	invoke("yes\n", true, "plugin", "oci", "image", "delete", "--unused", resource.ID)
	after := list()
	found := false
	for _, image := range after.Images {
		if selected[image.ID] {
			t.Fatal("deleted digest still present")
		}
		if image.ID == used && len(image.Containers) != 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("referenced image or container metadata lost")
	}
	// Explicit public deletion applies only to this test's freshly owned Store.
	invoke("yes\n", true, "plugin", "oci", "store", "delete", strings.TrimPrefix(resource.ID, "oci:"))
	t.Log("PASS real controller/CLI detached list, referenced refusal, reviewed unused candidates and confirmed deletion, canonical temporary cleanup and explicit owned Store cleanup")
	return true
}

// Diagnostics are observations, never authorization. Return only compiled labels;
// arbitrary CLI/backend text remains private even when it contains known tokens.
func maintenanceDiagnosticCategory(raw string) string {
	for _, code := range []string{"invalid_argument", "not_found", "already_exists", "unsupported", "unavailable", "busy", "denied", "incompatible_state", "recovery_required", "capability_stale", "internal"} {
		if strings.Contains(raw, "haco: "+code+":") {
			return code
		}
	}
	if strings.Contains(raw, "control endpoint unavailable") {
		return "endpoint_unavailable"
	}
	if strings.Contains(raw, "control protocol error") {
		return "protocol_error"
	}
	return "unclassified"
}

func maintenanceDiagnosticStages(raw string) string {
	var found []string
	for _, item := range []struct{ token, label string }{
		{"resolve Base ", "base_resolution"},
		{"workspace lock directory", "lifecycle_lock_directory"},
		{"not owned by effective uid", "lock_owner_mismatch"},
		{"ensure Incus project:", "project"},
		{"resolve isolated root storage:", "root_storage"},
		{"ensure Hacocoon routed sandbox substrate:", "routed_substrate"},
		{"resolve Hacocoon sandbox proxy configuration:", "sandbox_proxy"},
		{"init isolated Incus environment ", "instance_init"},
		{"invalid argument", "invalid_argument"},
		{"runtime unavailable", "runtime_unavailable"},
		{"capability request no longer matches current state", "stale_identity"},
		{"manual recovery required", "cleanup_incomplete"},
	} {
		if strings.Contains(raw, item.token) {
			found = append(found, item.label)
		}
	}
	if len(found) == 0 {
		return "unclassified"
	}
	return strings.Join(found, ",")
}

func TestMaintenanceDiagnosticsNeverReturnExternalText(t *testing.T) {
	for _, raw := range []string{
		"Authorization: Bearer synthetic-secret",
		"haco: internal: resolve Base synthetic-secret from https://user:password@example.invalid: runtime unavailable",
		"haco: unknown-secret: arbitrary output",
		"haco: invalid_argument: invalid argument\nsecret payload",
	} {
		category, stages := maintenanceDiagnosticCategory(raw), maintenanceDiagnosticStages(raw)
		if strings.Contains(category+stages, "secret") || strings.Contains(category+stages, "password") || strings.Contains(category+stages, "https") {
			t.Fatal("external diagnostic escaped")
		}
	}
	if got := maintenanceDiagnosticCategory("haco: internal: private detail"); got != "internal" {
		t.Fatal(got)
	}
	if got := maintenanceDiagnosticStages("resolve Base private detail: runtime unavailable"); got != "base_resolution,runtime_unavailable" {
		t.Fatal(got)
	}
	if got := maintenanceDiagnosticCategory("haco: arbitrary-secret: detail"); got != "unclassified" {
		t.Fatal(got)
	}
}
