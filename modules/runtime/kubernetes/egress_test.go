package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestResolveRuntimeRefRequiresOwnedPodAndNamespace(t *testing.T) {
	namespace := namespaceState{}
	namespace.Metadata.Name = "haco-demo"
	namespace.Metadata.Labels = managedLabels("demo")
	namespaceJSON, _ := json.Marshal(namespace)
	pods := `{"items":[{"metadata":{"name":"environment","namespace":"haco-demo","labels":{"app.kubernetes.io/managed-by":"hacocoon","hacocoon.dev/role":"environment","hacocoon.dev/environment":"demo"}},"status":{"podIP":"10.244.0.23"}}]}`

	runner := &fakeRunner{run: func(_ context.Context, _ string, args []string) (host.Result, error) {
		switch {
		case reflect.DeepEqual(args, []string{"get", "pods", "-A", "-l", sourcePodSelector, "-o", "json"}):
			return host.Result{Stdout: pods}, nil
		case reflect.DeepEqual(args, []string{"get", "namespace", "haco-demo", "--ignore-not-found", "-o", "json"}):
			return host.Result{Stdout: string(namespaceJSON)}, nil
		default:
			return host.Result{}, errors.New("unexpected kubectl call")
		}
	}}
	provider, err := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := provider.ResolveRuntimeRef(context.Background(), net.ParseIP("10.244.0.23"))
	if err != nil {
		t.Fatal(err)
	}
	if ref != "haco-demo" {
		t.Fatalf("ref = %q", ref)
	}
}

func TestResolveRuntimeRefRejectsUnownedNamespace(t *testing.T) {
	namespace := namespaceState{}
	namespace.Metadata.Name = "haco-demo"
	namespace.Metadata.Labels = managedLabels("demo")
	namespace.Metadata.Labels[managedByLabel] = "attacker"
	namespaceJSON, _ := json.Marshal(namespace)
	pods := `{"items":[{"metadata":{"name":"environment","namespace":"haco-demo","labels":{"app.kubernetes.io/managed-by":"hacocoon","hacocoon.dev/role":"environment","hacocoon.dev/environment":"demo"}},"status":{"podIP":"10.244.0.23"}}]}`

	runner := &fakeRunner{run: func(_ context.Context, _ string, args []string) (host.Result, error) {
		if args[1] == "pods" {
			return host.Result{Stdout: pods}, nil
		}
		return host.Result{Stdout: string(namespaceJSON)}, nil
	}}
	provider, _ := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	_, err := provider.ResolveRuntimeRef(context.Background(), net.ParseIP("10.244.0.23"))
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveRuntimeRefRejectsAmbiguousPodIP(t *testing.T) {
	pods := `{"items":[
		{"metadata":{"name":"environment","namespace":"haco-one","labels":{"app.kubernetes.io/managed-by":"hacocoon","hacocoon.dev/role":"environment","hacocoon.dev/environment":"one"}},"status":{"podIP":"10.244.0.23"}},
		{"metadata":{"name":"environment","namespace":"haco-two","labels":{"app.kubernetes.io/managed-by":"hacocoon","hacocoon.dev/role":"environment","hacocoon.dev/environment":"two"}},"status":{"podIP":"10.244.0.23"}}
	]}`
	runner := &fakeRunner{run: func(_ context.Context, _ string, args []string) (host.Result, error) {
		if args[0] == "get" && args[1] == "pods" {
			return host.Result{Stdout: pods}, nil
		}
		name := args[2]
		environment := "one"
		if name == "haco-two" {
			environment = "two"
		}
		state := namespaceState{}
		state.Metadata.Name = name
		state.Metadata.Labels = managedLabels(environment)
		data, _ := json.Marshal(state)
		return host.Result{Stdout: string(data)}, nil
	}}
	provider, _ := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	_, err := provider.ResolveRuntimeRef(context.Background(), net.ParseIP("10.244.0.23"))
	if !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatalf("error = %v", err)
	}
}

func TestResolveRuntimeRefRejectsMissingOrUnsafeSource(t *testing.T) {
	provider, _ := New(&fakeRunner{}, Config{Image: "example.invalid/hacocoon/systemd:26.04"})
	for _, source := range []net.IP{nil, net.ParseIP("127.0.0.1"), net.ParseIP("0.0.0.0"), net.ParseIP("224.0.0.1")} {
		if _, err := provider.ResolveRuntimeRef(context.Background(), source); !errors.Is(err, core.ErrPolicyDenied) {
			t.Fatalf("source %v error = %v", source, err)
		}
	}
}
