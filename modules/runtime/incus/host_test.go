package incus

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnsureTrustedHostCreatesMarkedInstanceWithNarrowControlProxyAndStartsIt(t *testing.T) {
	deviceAdded := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "storage" && args[1] == "show":
			return host.Result{}, nil
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			return host.Result{Stdout: "[]\n"}, nil
		case isConfigGet(args, trustedHostRoleKey):
			return host.Result{Stdout: trustedHostRoleValue + "\n"}, nil
		case isConfigGet(args, trustedHostControlEnvKey):
			return host.Result{Stdout: trustedHostControlSocket + "\n"}, nil
		case isDeviceList(args):
			if deviceAdded {
				return host.Result{Stdout: trustedHostControlDevice + "\n"}, nil
			}
			return host.Result{}, nil
		case isDeviceAdd(args):
			deviceAdded = true
			return host.Result{}, nil
		case isDeviceGet(args):
			return host.Result{Stdout: trustedHostDeviceValue(args[5], defaultPhysicalHostControlSocket) + "\n"}, nil
		default:
			return host.Result{}, nil
		}
	}}
	runtime := New(runner)
	if err := runtime.ConfigureStorageProvider(func(context.Context) (map[string]string, error) {
		return map[string]string{
			"incus_pool": "haco-local-default",
			"driver":     "btrfs",
			"source":     "/var/lib/hacocoon/mnt/local-default",
		}, nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := runtime.EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}

	assertCallContaining(t, runner.calls, "incus", []string{
		"init", defaultImage, trustedHostName,
		"--project", defaultProject,
		"--storage", "haco-local-default",
		"--config", trustedHostRoleKey + "=" + trustedHostRoleValue,
		"--config", trustedHostControlEnvKey + "=" + trustedHostControlSocket,
	})
	assertCallContaining(t, runner.calls, "incus", []string{
		"config", "device", "add", trustedHostName, trustedHostControlDevice, "proxy",
		"bind=instance",
		"listen=unix:" + trustedHostControlSocket,
		"connect=unix:" + defaultPhysicalHostControlSocket,
		"mode=0600", "uid=0", "gid=0",
		"--project", defaultProject,
	})
	assertCallContaining(t, runner.calls, "incus", []string{"start", trustedHostName, "--project", defaultProject})
}

func TestEnsureTrustedHostReusesRunningOwnedInstance(t *testing.T) {
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	if err := New(runner).EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && (call.args[0] == "init" || call.args[0] == "start" || isDeviceAdd(call.args)) {
			t.Fatalf("unexpected mutation for running converged host: %#v", call)
		}
	}
}

func TestEnsureTrustedHostStartsStoppedOwnedInstance(t *testing.T) {
	runner := trustedHostRunner("STOPPED", trustedHostRoleValue, nil)
	if err := New(runner).EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}
	assertCallContaining(t, runner.calls, "incus", []string{"start", trustedHostName, "--project", defaultProject})
}

func TestEnsureTrustedHostRefusesUnownedNameCollision(t *testing.T) {
	runner := trustedHostRunner("RUNNING", "", nil)
	err := New(runner).EnsureTrustedHost(context.Background())
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && (call.args[0] == "init" || call.args[0] == "start" || isDeviceAdd(call.args)) {
			t.Fatalf("unowned instance was mutated: %#v", call)
		}
	}
}

func TestEnsureTrustedHostRejectsUnexpectedState(t *testing.T) {
	runner := trustedHostRunner("FROZEN", trustedHostRoleValue, nil)
	err := New(runner).EnsureTrustedHost(context.Background())
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureTrustedHostRejectsUnexpectedControlProxy(t *testing.T) {
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, map[string]string{
		"connect": "unix:/tmp/not-hacocoon.sock",
	})
	err := New(runner).EnsureTrustedHost(context.Background())
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && (call.args[0] == "start" || isDeviceAdd(call.args)) {
			t.Fatalf("mismatched proxy was mutated/adopted: %#v", call)
		}
	}
}

func TestEnsureTrustedHostAddsMissingControlEnvironment(t *testing.T) {
	configured := false
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, call int, name string, args []string) (host.Result, error) {
		if isConfigGet(args, trustedHostControlEnvKey) {
			if configured {
				return host.Result{Stdout: trustedHostControlSocket + "\n"}, nil
			}
			return host.Result{}, nil
		}
		if len(args) >= 4 && args[0] == "config" && args[1] == "set" && args[2] == trustedHostName && args[3] == trustedHostControlEnvKey+"="+trustedHostControlSocket {
			configured = true
			return host.Result{}, nil
		}
		return original(ctx, call, name, args)
	}

	if err := New(runner).EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("control environment was not reconciled")
	}
}

func TestEnsureTrustedHostRejectsUnexpectedControlEnvironment(t *testing.T) {
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, call int, name string, args []string) (host.Result, error) {
		if isConfigGet(args, trustedHostControlEnvKey) {
			return host.Result{Stdout: "/tmp/other.sock\n"}, nil
		}
		return original(ctx, call, name, args)
	}
	err := New(runner).EnsureTrustedHost(context.Background())
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureTrustedHostRecoversConcurrentCreateOfOwnedInstance(t *testing.T) {
	initErr := errors.New("instance already exists")
	listCalls := 0
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, call int, name string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			listCalls++
			if listCalls == 1 {
				return host.Result{Stdout: "[]\n"}, nil
			}
			return host.Result{Stdout: `[{"name":"haco-host","status":"RUNNING"}]`}, nil
		case len(args) > 0 && args[0] == "init":
			return host.Result{ExitCode: 1}, initErr
		default:
			return original(ctx, call, name, args)
		}
	}

	if err := New(runner).EnsureTrustedHost(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestProvisionTrustedHostClientPushesAndVerifiesExactBinary(t *testing.T) {
	source := writeTrustedClientFixture(t, 0o755)
	payload, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	pushed := false

	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, call int, name string, args []string) (host.Result, error) {
		if len(args) >= 7 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "sha256sum" {
			if !pushed {
				return host.Result{ExitCode: 1}, errors.New("missing")
			}
			return host.Result{Stdout: digest + "  " + trustedHostClientPath + "\n"}, nil
		}
		if len(args) >= 7 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "stat" {
			return host.Result{Stdout: "755:0:0\n"}, nil
		}
		if len(args) >= 4 && args[0] == "file" && args[1] == "push" {
			pushed = true
			return host.Result{}, nil
		}
		return original(ctx, call, name, args)
	}

	if err := New(runner).ProvisionTrustedHostClient(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if !pushed {
		t.Fatal("client binary was not pushed")
	}
	assertCallContaining(t, runner.calls, "incus", []string{
		"file", "push", source, trustedHostName + trustedHostClientPath,
		"--project", defaultProject, "--create-dirs", "--uid", "0", "--gid", "0", "--mode", "0755",
	})
}

func TestProvisionTrustedHostClientSkipsPushWhenDigestAndOwnershipMatch(t *testing.T) {
	source := writeTrustedClientFixture(t, 0o755)
	payload, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))

	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, call int, name string, args []string) (host.Result, error) {
		if len(args) >= 7 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "sha256sum" {
			return host.Result{Stdout: digest + "  " + trustedHostClientPath + "\n"}, nil
		}
		if len(args) >= 7 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "stat" {
			return host.Result{Stdout: "755:0:0\n"}, nil
		}
		return original(ctx, call, name, args)
	}

	if err := New(runner).ProvisionTrustedHostClient(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 2 && call.args[0] == "file" && call.args[1] == "push" {
			t.Fatalf("idempotent provisioning unexpectedly pushed: %#v", call)
		}
	}
}

func TestProvisionTrustedHostClientRejectsWritableSource(t *testing.T) {
	source := writeTrustedClientFixture(t, 0o777)
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	err := New(runner).ProvisionTrustedHostClient(context.Background(), source)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 2 && call.args[0] == "file" && call.args[1] == "push" {
			t.Fatalf("unsafe source was pushed: %#v", call)
		}
	}
}

func trustedHostRunner(state, role string, overrides map[string]string) *fakeRunner {
	return &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "profile" && args[1] == "show":
			return rootProfileResult(), nil
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			return host.Result{Stdout: `[{"name":"haco-host","status":"` + state + `"}]`}, nil
		case isConfigGet(args, trustedHostRoleKey):
			return host.Result{Stdout: role + "\n"}, nil
		case isConfigGet(args, trustedHostControlEnvKey):
			return host.Result{Stdout: trustedHostControlSocket + "\n"}, nil
		case isDeviceList(args):
			return host.Result{Stdout: trustedHostControlDevice + "\n"}, nil
		case isDeviceGet(args):
			value := trustedHostDeviceValue(args[5], defaultPhysicalHostControlSocket)
			if override, ok := overrides[args[5]]; ok {
				value = override
			}
			return host.Result{Stdout: value + "\n"}, nil
		default:
			return host.Result{}, nil
		}
	}}
}

func trustedHostDeviceValue(key, hostSocket string) string {
	switch key {
	case "type":
		return "proxy"
	case "bind":
		return "instance"
	case "listen":
		return "unix:" + trustedHostControlSocket
	case "connect":
		return "unix:" + hostSocket
	case "mode":
		return "0600"
	case "uid", "gid":
		return "0"
	case "nat", "proxy_protocol", "security.uid", "security.gid":
		return ""
	default:
		return ""
	}
}

func isConfigGet(args []string, key string) bool {
	return len(args) >= 4 && args[0] == "config" && args[1] == "get" && args[2] == trustedHostName && args[3] == key
}

func isDeviceList(args []string) bool {
	return len(args) >= 4 && args[0] == "config" && args[1] == "device" && args[2] == "list" && args[3] == trustedHostName
}

func isDeviceAdd(args []string) bool {
	return len(args) >= 6 && args[0] == "config" && args[1] == "device" && args[2] == "add" && args[3] == trustedHostName && args[4] == trustedHostControlDevice
}

func isDeviceGet(args []string) bool {
	return len(args) >= 6 && args[0] == "config" && args[1] == "device" && args[2] == "get" && args[3] == trustedHostName && args[4] == trustedHostControlDevice
}

func assertCallContaining(t *testing.T, calls []runnerCall, name string, wantArgs []string) {
	t.Helper()
	for _, call := range calls {
		if call.name != name || len(call.args) != len(wantArgs) {
			continue
		}
		matched := true
		for i := range wantArgs {
			if call.args[i] != wantArgs[i] {
				matched = false
				break
			}
		}
		if matched {
			return
		}
	}
	t.Fatalf("missing call %s %s\ncalls=%#v", name, strings.Join(wantArgs, " "), calls)
}

func writeTrustedClientFixture(t *testing.T, mode os.FileMode) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "haco-host")
	if err := os.WriteFile(path, []byte("test-client-binary\n"), mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatal(err)
	}
	return path
}
