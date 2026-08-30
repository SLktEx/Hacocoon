package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls []runnerCall
	run   func(context.Context, string, []string) (host.Result, error)
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	copyArgs := append([]string(nil), args...)
	f.calls = append(f.calls, runnerCall{name: name, args: copyArgs})
	if f.run != nil {
		return f.run(ctx, name, copyArgs)
	}
	return host.Result{}, nil
}

func TestWorkloadManifestLocksDownKubernetesAuthority(t *testing.T) {
	manifest := workloadManifest(
		"haco-demo",
		"demo",
		"/srv/work/demo",
		false,
		"example.invalid/hacocoon/systemd:26.04",
		"sysbox-runc",
		core.UnlimitedResourceBudget(),
	)
	items := manifest["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}

	policy := itemByKind(t, items, "NetworkPolicy")
	policySpec := policy["spec"].(map[string]any)
	if got := policySpec["policyTypes"]; !reflect.DeepEqual(got, []string{"Ingress", "Egress"}) {
		t.Fatalf("policyTypes = %#v", got)
	}
	if _, ok := policySpec["ingress"]; ok {
		t.Fatal("default-deny policy unexpectedly contains ingress allow rules")
	}
	if _, ok := policySpec["egress"]; ok {
		t.Fatal("default-deny policy unexpectedly contains egress allow rules")
	}

	pod := itemByKind(t, items, "Pod")
	spec := pod["spec"].(map[string]any)
	assertEqual(t, spec["runtimeClassName"], "sysbox-runc", "runtimeClassName")
	assertEqual(t, spec["hostUsers"], false, "hostUsers")
	assertEqual(t, spec["automountServiceAccountToken"], false, "automountServiceAccountToken")
	assertEqual(t, spec["hostNetwork"], false, "hostNetwork")
	assertEqual(t, spec["hostPID"], false, "hostPID")
	assertEqual(t, spec["hostIPC"], false, "hostIPC")

	containers := spec["containers"].([]any)
	container := containers[0].(map[string]any)
	security := container["securityContext"].(map[string]any)
	assertEqual(t, security["privileged"], false, "privileged")
	if _, ok := container["env"]; ok {
		t.Fatal("Environment Pod unexpectedly contains environment variables that could become ambient credentials")
	}

	volumes := spec["volumes"].([]any)
	if len(volumes) != 1 {
		t.Fatalf("volumes = %#v", volumes)
	}
	hostPath := volumes[0].(map[string]any)["hostPath"].(map[string]any)
	assertEqual(t, hostPath["path"], "/srv/work/demo", "workspace hostPath")
}

func TestCreateEnvironmentUsesSecureManifestAndVerifiesSystemd(t *testing.T) {
	var manifests [][]byte
	runner := &fakeRunner{}
	runner.run = func(_ context.Context, _ string, args []string) (host.Result, error) {
		switch {
		case reflect.DeepEqual(args, []string{"get", "namespace", "haco-demo", "--ignore-not-found", "-o", "json"}):
			return host.Result{}, nil
		case len(args) == 3 && args[0] == "create" && args[1] == "-f":
			data, err := os.ReadFile(args[2])
			if err != nil {
				return host.Result{}, err
			}
			manifests = append(manifests, data)
			return host.Result{}, nil
		case len(args) >= 4 && args[0] == "-n" && args[2] == "wait":
			return host.Result{}, nil
		case reflect.DeepEqual(args, []string{"-n", "haco-demo", "exec", podName, "--", "cat", "/proc/1/comm"}):
			return host.Result{Stdout: "systemd\n"}, nil
		case reflect.DeepEqual(args, []string{"-n", "haco-demo", "exec", podName, "--", "test", "-w", "/workspace"}):
			return host.Result{}, nil
		default:
			return host.Result{}, errors.New("unexpected kubectl call")
		}
	}
	provider, err := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	if err != nil {
		t.Fatal(err)
	}

	created, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/srv/work/demo",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Ref != "haco-demo" {
		t.Fatalf("ref = %q", created.Ref)
	}
	if len(manifests) != 2 {
		t.Fatalf("manifests = %d, want namespace + workload", len(manifests))
	}
	var namespace map[string]any
	if err := json.Unmarshal(manifests[0], &namespace); err != nil {
		t.Fatal(err)
	}
	labels := namespace["metadata"].(map[string]any)["labels"].(map[string]any)
	assertEqual(t, labels[managedByLabel], managedByValue, "namespace ownership")
}

func TestExecRejectsNamespaceWithoutExactOwnership(t *testing.T) {
	state := `{"metadata":{"name":"haco-demo","labels":{"app.kubernetes.io/managed-by":"someone-else","hacocoon.dev/role":"environment","hacocoon.dev/environment":"demo"}}}`
	runner := &fakeRunner{run: func(_ context.Context, _ string, args []string) (host.Result, error) {
		if reflect.DeepEqual(args, []string{"get", "namespace", "haco-demo", "--ignore-not-found", "-o", "json"}) {
			return host.Result{Stdout: state}, nil
		}
		return host.Result{}, errors.New("unexpected mutation")
	}}
	provider, err := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.ExecEnvironment(context.Background(), "haco-demo", core.ExecutionRequest{Argv: []string{"id"}})
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
}

func TestExecPreservesArgumentBoundaries(t *testing.T) {
	state := `{"metadata":{"name":"haco-demo","labels":{"app.kubernetes.io/managed-by":"hacocoon","hacocoon.dev/role":"environment","hacocoon.dev/environment":"demo"}}}`
	runner := &fakeRunner{run: func(_ context.Context, _ string, args []string) (host.Result, error) {
		if args[0] == "get" {
			return host.Result{Stdout: state}, nil
		}
		return host.Result{ExitCode: 17, Stdout: "out", Stderr: "err"}, errors.New("exit 17")
	}}
	provider, err := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := provider.ExecEnvironment(context.Background(), "haco-demo", core.ExecutionRequest{Argv: []string{"printf", "%s", "hello world", "--help"}})
	if err == nil || result.ExitCode != 17 {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	want := []string{"-n", "haco-demo", "exec", podName, "--", "printf", "%s", "hello world", "--help"}
	if !reflect.DeepEqual(runner.calls[1].args, want) {
		t.Fatalf("exec args = %#v, want %#v", runner.calls[1].args, want)
	}
}

func TestCreateRejectsReservedOrInjectedNamesBeforeKubectl(t *testing.T) {
	provider, err := New(&fakeRunner{}, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"host", "Demo", "demo;whoami", "demo/other", "-demo", "demo-"} {
		_, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: name, WorkspacePath: "/srv/work/demo"})
		if !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("name %q error = %v", name, err)
		}
	}
}

func TestFinitePIDBudgetFailsBeforeClusterMutation(t *testing.T) {
	runner := &fakeRunner{}
	provider, err := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/srv/work/demo",
		Resources: core.ResourceBudget{
			PIDs: core.ResourceLimit{Mode: core.ResourceLimitFinite, Value: 100},
		},
	})
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("cluster was touched: %#v", runner.calls)
	}
}

func itemByKind(t *testing.T, items []any, kind string) map[string]any {
	t.Helper()
	for _, raw := range items {
		item := raw.(map[string]any)
		if item["kind"] == kind {
			return item
		}
	}
	t.Fatalf("kind %q not found", kind)
	return nil
}

func assertEqual(t *testing.T, got, want any, name string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}
