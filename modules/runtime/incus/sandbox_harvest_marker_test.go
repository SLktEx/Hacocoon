package incus

import (
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestSandboxProviderMarksEnvironmentForSeedHarvestBeforeStart(t *testing.T) {
	values := map[string]string{}
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "image" && args[1] == "info" {
			return host.Result{Stdout: `{"fingerprint":"` + sandboxTestFingerprint + `"}`}, nil
		}
		if result, ok := sandboxNetworkResult(args); ok {
			return result, nil
		}
		if len(args) >= 3 && args[0] == "profile" && args[1] == "show" && args[2] == "default" {
			return rootProfileResult(), nil
		}
		if len(args) >= 4 && args[0] == "config" && args[1] == "set" {
			parts := strings.SplitN(args[3], "=", 2)
			values[parts[0]] = parts[1]
			return host.Result{}, nil
		}
		if len(args) >= 4 && args[0] == "config" && args[1] == "get" {
			return host.Result{Stdout: values[args[3]] + "\n"}, nil
		}
		return host.Result{}, nil
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.CreateEnvironment(context.Background(), core.EnvironmentRuntimeSpec{Name: "harvest", WorkspacePath: "/tmp/work"}); err != nil {
		t.Fatal(err)
	}
	if values[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue {
		t.Fatalf("marker=%q want=%q", values[managedEnvironmentMarkerKey], managedEnvironmentMarkerValue)
	}
	markerSet := -1
	start := -1
	for i, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, managedEnvironmentMarkerKey+"="+managedEnvironmentMarkerValue) {
			markerSet = i
		}
		if len(call.args) > 0 && call.args[0] == "start" {
			start = i
		}
	}
	if markerSet < 0 || start < 0 || markerSet >= start {
		t.Fatalf("marker-set=%d start=%d calls=%#v", markerSet, start, runner.calls)
	}
}
