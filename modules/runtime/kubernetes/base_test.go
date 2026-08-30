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

const (
	testBaseDigestA = "1111111111111111111111111111111111111111111111111111111111111111"
	testBaseDigestB = "2222222222222222222222222222222222222222222222222222222222222222"
)

func TestKubernetesBaseListAndInspectUseImmutableDigest(t *testing.T) {
	provider, err := New(&fakeRunner{}, Config{Image: "example.invalid/hacocoon/ubuntu@sha256:" + testBaseDigestA})
	if err != nil {
		t.Fatal(err)
	}
	bases, err := NewBaseProvider(provider)
	if err != nil {
		t.Fatal(err)
	}

	listed, err := bases.ListBases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(listed, []core.BaseInfo{{Name: defaultKubernetesBaseName}}) {
		t.Fatalf("bases = %#v", listed)
	}
	inspected, err := bases.InspectBase(context.Background(), defaultKubernetesBaseName)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Name != defaultKubernetesBaseName || inspected.Revision != core.BaseRevision("sha256:"+testBaseDigestA) {
		t.Fatalf("base = %#v", inspected)
	}
}

func TestKubernetesBaseCreatePinsExactImageAndRevision(t *testing.T) {
	var workloadImage string
	namespaces := map[string]string{}
	runner := &fakeRunner{}
	runner.run = func(_ context.Context, _ string, args []string) (host.Result, error) {
		switch {
		case reflect.DeepEqual(args, []string{"get", "namespace", "haco-demo", "--ignore-not-found", "-o", "json"}):
			if _, ok := namespaces["haco-demo"]; !ok {
				return host.Result{}, nil
			}
			state := namespaceState{}
			state.Metadata.Name = "haco-demo"
			state.Metadata.Labels = managedLabels("demo")
			data, _ := json.Marshal(state)
			return host.Result{Stdout: string(data)}, nil
		case len(args) == 3 && args[0] == "create" && args[1] == "-f":
			data, err := os.ReadFile(args[2])
			if err != nil {
				return host.Result{}, err
			}
			var manifest map[string]any
			if err := json.Unmarshal(data, &manifest); err != nil {
				return host.Result{}, err
			}
			if manifest["kind"] == "Namespace" {
				namespaces["haco-demo"] = "demo"
				return host.Result{}, nil
			}
			if manifest["kind"] == "List" {
				for _, raw := range manifest["items"].([]any) {
					item := raw.(map[string]any)
					if item["kind"] != "Pod" {
						continue
					}
					containers := item["spec"].(map[string]any)["containers"].([]any)
					workloadImage = containers[0].(map[string]any)["image"].(string)
				}
				return host.Result{}, nil
			}
			return host.Result{}, errors.New("unexpected manifest")
		case len(args) >= 5 && args[0] == "-n" && args[2] == "wait":
			return host.Result{}, nil
		case reflect.DeepEqual(args, []string{"-n", "haco-demo", "exec", podName, "--", "cat", "/proc/1/comm"}):
			return host.Result{Stdout: "systemd\n"}, nil
		case reflect.DeepEqual(args, []string{"-n", "haco-demo", "exec", podName, "--", "test", "-w", "/workspace"}):
			return host.Result{}, nil
		default:
			return host.Result{}, errors.New("unexpected kubectl call")
		}
	}
	image := "example.invalid/hacocoon/ubuntu@sha256:" + testBaseDigestA
	provider, err := New(runner, Config{Image: image})
	if err != nil {
		t.Fatal(err)
	}
	bases, err := NewBaseProvider(provider)
	if err != nil {
		t.Fatal(err)
	}
	created, err := bases.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/srv/work/demo",
		Base:          defaultKubernetesBaseName,
	})
	if err != nil {
		t.Fatal(err)
	}
	if workloadImage != image {
		t.Fatalf("workload image = %q, want %q", workloadImage, image)
	}
	if created.Base == nil || created.Base.Name != defaultKubernetesBaseName || created.Base.Revision != core.BaseRevision("sha256:"+testBaseDigestA) {
		t.Fatalf("created Base = %#v", created.Base)
	}
}

func TestKubernetesBaseRejectsMutableTagBeforeClusterMutation(t *testing.T) {
	runner := &fakeRunner{}
	provider, err := New(runner, Config{Image: "example.invalid/hacocoon/ubuntu:26.04"})
	if err != nil {
		t.Fatal(err)
	}
	bases, err := NewBaseProvider(provider)
	if err != nil {
		t.Fatal(err)
	}
	_, err = bases.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{
		Name:          "demo",
		WorkspacePath: "/srv/work/demo",
	})
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("mutable Base touched Kubernetes: %#v", runner.calls)
	}
}

func TestKubernetesCustomBaseConfigurationIsImmutableAndReserved(t *testing.T) {
	t.Setenv(kubernetesBasesConfigEnv, `{"team/dev":"registry.example/dev@sha256:`+testBaseDigestB+`"}`)
	provider, err := New(&fakeRunner{}, Config{Image: "example.invalid/hacocoon/ubuntu@sha256:" + testBaseDigestA})
	if err != nil {
		t.Fatal(err)
	}
	bases, err := NewBaseProvider(provider)
	if err != nil {
		t.Fatal(err)
	}
	inspected, err := bases.InspectBase(context.Background(), core.BaseName("team/dev"))
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Revision != core.BaseRevision("sha256:"+testBaseDigestB) {
		t.Fatalf("revision = %q", inspected.Revision)
	}

	t.Setenv(kubernetesBasesConfigEnv, `{"haco/ubuntu-26.04":"registry.example/override@sha256:`+testBaseDigestB+`"}`)
	if _, err := NewBaseProvider(provider); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("reserved Base override error = %v", err)
	}
}

func TestKubernetesBaseMissingNameFailsNotFound(t *testing.T) {
	provider, err := New(&fakeRunner{}, Config{Image: "example.invalid/hacocoon/ubuntu@sha256:" + testBaseDigestA})
	if err != nil {
		t.Fatal(err)
	}
	bases, err := NewBaseProvider(provider)
	if err != nil {
		t.Fatal(err)
	}
	_, err = bases.InspectBase(context.Background(), core.BaseName("team/missing"))
	if !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("error = %v", err)
	}
}
