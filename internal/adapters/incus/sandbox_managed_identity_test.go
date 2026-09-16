package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestSandboxProviderMarksManagedEnvironmentBeforeStart(t *testing.T) {
	for _, mode := range []string{"writable", "read-only", "write-probe-failure"} {
		t.Run(mode, func(t *testing.T) {
			values := map[string]string{}
			writeProbes := 0
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] == "exec" && strings.HasSuffix(strings.Join(args, " "), "-- test -w /workspace") {
					writeProbes++
					if mode == "write-probe-failure" {
						return host.Result{ExitCode: 1, Stderr: "permission denied"}, errors.New("unwritable workspace")
					}
				}
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
			var recorded core.EnvironmentRuntime
			created, err := provider.CreateEnvironmentWithReceipt(context.Background(), core.EnvironmentRuntimeSpec{Name: "demo", WorkspacePath: "/tmp/work", InstanceID: testEnvironmentInstance, ReadOnly: mode == "read-only"}, func(value core.EnvironmentRuntime) error { recorded = value; return nil })
			if (err == nil) != (mode != "write-probe-failure") || created.Ref != "haco-demo" || recorded.Ref != created.Ref {
				t.Fatal("provider lost creation receipt", created, recorded, err)
			}
			if mode == "write-probe-failure" && !errors.Is(err, core.ErrUnsupported) {
				t.Fatal("unwritable Workspace was accepted", err)
			}
			if (writeProbes == 0) != (mode == "read-only") {
				t.Fatal("write probe ignored the selected access mode", mode, writeProbes)
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
				if len(call.args) > 0 && call.args[0] == "delete" {
					t.Fatal("provider deleted the receipt owned by canonical lifecycle", call)
				}
			}
			if markerSet < 0 || start < 0 || markerSet >= start {
				t.Fatalf("marker-set=%d start=%d calls=%#v", markerSet, start, runner.calls)
			}
		})
	}
}
