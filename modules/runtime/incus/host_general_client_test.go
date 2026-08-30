package incus

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestProvisionTrustedHostGeneralClientSetsModePushesAndVerifies(t *testing.T) {
	source := writeTrustedClientFixture(t, 0o755)
	payload, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))
	configured := false
	pushed := false

	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, call int, name string, args []string) (host.Result, error) {
		switch {
		case isConfigGet(args, trustedHostClientModeEnvKey):
			if configured {
				return host.Result{Stdout: trustedHostClientModeValue + "\n"}, nil
			}
			return host.Result{}, nil
		case len(args) >= 4 && args[0] == "config" && args[1] == "set" && args[2] == trustedHostName && args[3] == trustedHostClientModeEnvKey+"="+trustedHostClientModeValue:
			configured = true
			return host.Result{}, nil
		case len(args) >= 7 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "sha256sum" && args[6] == trustedHostGeneralClientPath:
			if !pushed {
				return host.Result{ExitCode: 1}, errors.New("missing")
			}
			return host.Result{Stdout: digest + "  " + trustedHostGeneralClientPath + "\n"}, nil
		case len(args) >= 9 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "stat" && args[8] == trustedHostGeneralClientPath:
			return host.Result{Stdout: "755:0:0\n"}, nil
		case len(args) >= 4 && args[0] == "file" && args[1] == "push" && args[3] == trustedHostName+trustedHostGeneralClientPath:
			pushed = true
			return host.Result{}, nil
		default:
			return original(ctx, call, name, args)
		}
	}

	if err := New(runner).ProvisionTrustedHostGeneralClient(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if !configured {
		t.Fatal("controller client mode was not reconciled")
	}
	if !pushed {
		t.Fatal("general haco client was not pushed")
	}
	assertCallContaining(t, runner.calls, "incus", []string{
		"file", "push", source, trustedHostName + trustedHostGeneralClientPath,
		"--project", defaultProject, "--create-dirs", "--uid", "0", "--gid", "0", "--mode", "0755",
	})
}

func TestProvisionTrustedHostGeneralClientRejectsUnexpectedMode(t *testing.T) {
	source := writeTrustedClientFixture(t, 0o755)
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, call int, name string, args []string) (host.Result, error) {
		if isConfigGet(args, trustedHostClientModeEnvKey) {
			return host.Result{Stdout: "local\n"}, nil
		}
		return original(ctx, call, name, args)
	}

	err := New(runner).ProvisionTrustedHostGeneralClient(context.Background(), source)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 2 && call.args[0] == "file" && call.args[1] == "push" {
			t.Fatalf("mismatched client mode was mutated through push: %#v", call)
		}
	}
}

func TestProvisionTrustedHostGeneralClientSkipsPushWhenConverged(t *testing.T) {
	source := writeTrustedClientFixture(t, 0o755)
	payload, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(payload))

	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, call int, name string, args []string) (host.Result, error) {
		switch {
		case isConfigGet(args, trustedHostClientModeEnvKey):
			return host.Result{Stdout: trustedHostClientModeValue + "\n"}, nil
		case len(args) >= 7 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "sha256sum" && args[6] == trustedHostGeneralClientPath:
			return host.Result{Stdout: digest + "  " + trustedHostGeneralClientPath + "\n"}, nil
		case len(args) >= 9 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "stat" && args[8] == trustedHostGeneralClientPath:
			return host.Result{Stdout: "755:0:0\n"}, nil
		default:
			return original(ctx, call, name, args)
		}
	}

	if err := New(runner).ProvisionTrustedHostGeneralClient(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 2 && call.args[0] == "file" && call.args[1] == "push" {
			t.Fatalf("converged general client unexpectedly pushed: %#v", call)
		}
	}
}
