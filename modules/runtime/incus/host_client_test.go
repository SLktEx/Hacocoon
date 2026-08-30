package incus

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestProvisionTrustedHostClientInstallsBinaryAndAddsNarrowProxy(t *testing.T) {
	deviceMissing := errors.New("device not found")
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "profile" && args[1] == "show":
			return rootProfileResult(), nil
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			return host.Result{Stdout: `[{"name":"haco-host","status":"RUNNING"}]`}, nil
		case reflect.DeepEqual(args, []string{"config", "get", trustedHostName, trustedHostRoleKey, "--project", defaultProject}):
			return host.Result{Stdout: trustedHostRoleValue + "\n"}, nil
		case reflect.DeepEqual(args, []string{"config", "device", "get", trustedHostName, trustedHostControlDevice, "listen", "--project", defaultProject}):
			return host.Result{ExitCode: 1}, deviceMissing
		default:
			return host.Result{}, nil
		}
	}}

	err := New(runner).ProvisionTrustedHostClient(context.Background(), "/opt/haco/haco-host", "/run/hacocoon/control.sock")
	if err != nil {
		t.Fatal(err)
	}

	wantTail := []runnerCall{
		{name: "incus", args: []string{"config", "get", trustedHostName, trustedHostRoleKey, "--project", defaultProject}},
		{name: "incus", args: []string{"exec", trustedHostName, "--project", defaultProject, "--", "install", "-d", "-m", "0755", "/usr/local/bin", "/run/hacocoon"}},
		{name: "incus", args: []string{"file", "push", "/opt/haco/haco-host", trustedHostName + trustedHostClientPath, "--project", defaultProject, "--uid", "0", "--gid", "0", "--mode", "0755"}},
		{name: "incus", args: []string{"config", "device", "get", trustedHostName, trustedHostControlDevice, "listen", "--project", defaultProject}},
		{name: "incus", args: []string{"config", "device", "add", trustedHostName, trustedHostControlDevice, "proxy", "listen=unix:/run/hacocoon/control.sock", "connect=unix:/run/hacocoon/control.sock", "bind=instance", "uid=0", "gid=0", "mode=0600", "--project", defaultProject}},
		{name: "incus", args: []string{"exec", trustedHostName, "--project", defaultProject, "--", trustedHostClientPath, "doctor"}},
	}
	if len(runner.calls) < len(wantTail) {
		t.Fatalf("calls = %#v", runner.calls)
	}
	gotTail := runner.calls[len(runner.calls)-len(wantTail):]
	if !reflect.DeepEqual(gotTail, wantTail) {
		t.Fatalf("tail calls = %#v\nwant = %#v", gotTail, wantTail)
	}
}

func TestProvisionTrustedHostClientReusesExactProxy(t *testing.T) {
	values := map[string]string{
		"listen":  "unix:/run/hacocoon/control.sock\n",
		"connect": "unix:/run/hacocoon/control.sock\n",
		"bind":    "instance\n",
		"uid":     "0\n",
		"gid":     "0\n",
		"mode":    "600\n",
	}
	runner := provisionedTrustedHostRunner(values)
	if err := New(runner).ProvisionTrustedHostClient(context.Background(), "/opt/haco/haco-host", "/run/hacocoon/control.sock"); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 3 && call.args[0] == "config" && call.args[1] == "device" && call.args[2] == "add" {
			t.Fatalf("exact control proxy was unexpectedly replaced: %#v", call)
		}
	}
}

func TestProvisionTrustedHostClientRefusesMismatchedProxy(t *testing.T) {
	values := map[string]string{
		"listen":  "unix:/run/hacocoon/control.sock\n",
		"connect": "unix:/tmp/wrong.sock\n",
		"bind":    "instance\n",
		"uid":     "0\n",
		"gid":     "0\n",
		"mode":    "0600\n",
	}
	runner := provisionedTrustedHostRunner(values)
	err := New(runner).ProvisionTrustedHostClient(context.Background(), "/opt/haco/haco-host", "/run/hacocoon/control.sock")
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 3 && call.args[0] == "config" && call.args[1] == "device" && call.args[2] == "add" {
			t.Fatalf("mismatched control proxy was implicitly replaced: %#v", call)
		}
	}
}

func TestProvisionTrustedHostClientRejectsRelativeControllerSocket(t *testing.T) {
	err := New(&fakeRunner{}).ProvisionTrustedHostClient(context.Background(), "/opt/haco/haco-host", "relative.sock")
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("error = %v, want ErrInvalidArgument", err)
	}
}

func provisionedTrustedHostRunner(values map[string]string) *fakeRunner {
	return &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "profile" && args[1] == "show":
			return rootProfileResult(), nil
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			return host.Result{Stdout: `[{"name":"haco-host","status":"RUNNING"}]`}, nil
		case reflect.DeepEqual(args, []string{"config", "get", trustedHostName, trustedHostRoleKey, "--project", defaultProject}):
			return host.Result{Stdout: trustedHostRoleValue + "\n"}, nil
		case len(args) >= 7 && args[0] == "config" && args[1] == "device" && args[2] == "get" && args[3] == trustedHostName && args[4] == trustedHostControlDevice:
			return host.Result{Stdout: values[args[5]]}, nil
		default:
			return host.Result{}, nil
		}
	}}
}
