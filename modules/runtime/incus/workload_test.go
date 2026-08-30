package incus

import (
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestCreateWorkloadLaunchesIncusOCIWithoutNestedRuntime(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 6 && args[0] == "list" && args[1] == "haco-demo" && args[4] == "--format" {
			return host.Result{Stdout: "haco-demo\n"}, nil
		}
		if len(args) >= 5 && args[0] == "config" && args[1] == "get" && args[2] == "haco-demo" && args[3] == managedEnvironmentMarkerKey {
			return host.Result{Stdout: managedEnvironmentMarkerValue + "\n"}, nil
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		if len(args) >= 3 && args[0] == "profile" && args[1] == "show" && args[2] == "default" {
			return rootProfileResult(), nil
		}
		if len(args) >= 2 && args[0] == "list" && args[1] == "haco-w-demo-db" {
			return host.Result{}, nil
		}
		return host.Result{}, nil
	}}
	runtime := New(runner)
	created, err := runtime.CreateWorkload(context.Background(), core.WorkloadSpec{
		Environment: "demo",
		Name:        "db",
		Image:       "oci-docker:library/postgres:18",
		EnvironmentVariables: map[string]string{
			"POSTGRES_DB": "app",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.RuntimeRef != "haco-w-demo-db" || created.Image != "oci-docker:library/postgres:18" {
		t.Fatalf("created = %#v", created)
	}

	seenLaunch := false
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if call.name != "incus" {
			t.Fatalf("unexpected host command %q: %#v", call.name, call.args)
		}
		if strings.Contains(joined, "systemctl") || strings.Contains(joined, "nerdctl") || strings.Contains(joined, "containerd") {
			t.Fatalf("nested OCI runtime command leaked into Incus workload path: %s", joined)
		}
		if len(call.args) > 0 && call.args[0] == "launch" {
			seenLaunch = true
			for _, required := range []string{
				"oci-docker:library/postgres:18",
				"haco-w-demo-db",
				workloadKindKey + "=" + workloadKindValue,
				workloadEnvironmentKey + "=demo",
				workloadNameKey + "=db",
				"environment.POSTGRES_DB=app",
			} {
				if !strings.Contains(joined, required) {
					t.Fatalf("launch missing %q: %s", required, joined)
				}
			}
		}
	}
	if !seenLaunch {
		t.Fatal("Incus launch call missing")
	}
}

func TestWorkloadRefRejectsCrossScopeAndLongNames(t *testing.T) {
	for _, tc := range []struct {
		environment string
		name        string
	}{
		{"demo", "../db"},
		{"other_env", "db"},
		{"demo", strings.Repeat("a", 60)},
	} {
		if _, err := workloadRef(tc.environment, tc.name); err == nil {
			t.Fatalf("workloadRef(%q, %q) unexpectedly succeeded", tc.environment, tc.name)
		}
	}
}

func TestEncodeOCIEntrypointQuotesArguments(t *testing.T) {
	got, err := encodeOCIEntrypoint([]string{"sh", "-c", "echo 'hello world'"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "sh -c 'echo '\"'\"'hello world'\"'\"''" {
		t.Fatalf("entrypoint = %q", got)
	}
}
