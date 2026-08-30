package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnsureTrustedHostNerdctlShimCreatesOnlyMissingSymlink(t *testing.T) {
	linked := false
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			return host.Result{Stdout: `[{"name":"haco-host","status":"RUNNING"}]`}, nil
		case isConfigGet(args, trustedHostRoleKey):
			return host.Result{Stdout: trustedHostRoleValue + "\n"}, nil
		case len(args) >= 7 && args[0] == "exec" && args[5] == "readlink":
			if !linked {
				return host.Result{ExitCode: 1}, errors.New("missing")
			}
			return host.Result{Stdout: trustedHostClientPath + "\n"}, nil
		case len(args) >= 9 && args[0] == "exec" && args[5] == "test":
			return host.Result{}, nil
		case len(args) >= 9 && args[0] == "exec" && args[5] == "ln":
			linked = true
			return host.Result{}, nil
		default:
			return host.Result{}, nil
		}
	}}
	if err := New(runner).EnsureTrustedHostNerdctlShim(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !linked {
		t.Fatal("nerdctl shim symlink was not created")
	}
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "ln -sfn") || strings.Contains(joined, " rm ") {
			t.Fatalf("unsafe overwrite operation used: %s", joined)
		}
	}
}

func TestEnsureTrustedHostNerdctlShimRefusesExistingDifferentSymlink(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		switch {
		case len(args) >= 2 && args[0] == "list" && args[1] == trustedHostName:
			return host.Result{Stdout: `[{"name":"haco-host","status":"RUNNING"}]`}, nil
		case isConfigGet(args, trustedHostRoleKey):
			return host.Result{Stdout: trustedHostRoleValue + "\n"}, nil
		case len(args) >= 7 && args[0] == "exec" && args[5] == "readlink":
			return host.Result{Stdout: "/usr/bin/nerdctl\n"}, nil
		default:
			return host.Result{}, nil
		}
	}}
	err := New(runner).EnsureTrustedHostNerdctlShim(context.Background())
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) >= 6 && call.args[0] == "exec" && call.args[5] == "ln" {
			t.Fatalf("existing nerdctl path was overwritten: %#v", call)
		}
	}
}
