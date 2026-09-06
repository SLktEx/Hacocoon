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

func setupClientFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"haco-host", "haco"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("client-"+name), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestHostSetupRejectsInvalidCompanionsBeforeProviderMutation(t *testing.T) {
	for _, kind := range []string{"missing", "symlink", "writable", "not-executable"} {
		t.Run(kind, func(t *testing.T) {
			dir := setupClientFixtures(t)
			path := filepath.Join(dir, "haco")
			switch kind {
			case "missing":
				_ = os.Remove(path)
			case "symlink":
				_ = os.Remove(path)
				if err := os.Symlink(filepath.Join(dir, "haco-host"), path); err != nil {
					t.Fatal(err)
				}
			case "writable":
				_ = os.Chmod(path, 0777)
			case "not-executable":
				_ = os.Chmod(path, 0644)
			}
			runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
			if err := New(runner).SetupTrustedHost(context.Background(), dir); err == nil {
				t.Fatal("accepted unsafe companion")
			}
			if len(runner.calls) != 0 {
				t.Fatalf("provider mutated before source validation: %v", runner.calls)
			}
		})
	}
}

func TestHostSetupReusesOwnedHostAndRecoversPartialClientInstall(t *testing.T) {
	dir := setupClientFixtures(t)
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	installed := map[string]string{}
	pushes := map[string]int{}
	failProduct := true
	runner.run = func(ctx context.Context, n int, name string, args []string) (host.Result, error) {
		if isConfigGet(args, trustedHostClientModeEnvKey) {
			return host.Result{Stdout: trustedHostClientModeValue}, nil
		}
		if len(args) >= 4 && args[0] == "file" && args[1] == "push" {
			target := strings.TrimPrefix(args[3], trustedHostName)
			pushes[target]++
			if target == trustedHostProductClientPath && failProduct {
				return host.Result{}, errors.New("interrupted install")
			}
			data, err := os.ReadFile(args[2])
			if err != nil {
				return host.Result{}, err
			}
			installed[target] = fmt.Sprintf("%x", sha256.Sum256(data))
			return host.Result{}, nil
		}
		if len(args) >= 7 && args[0] == "exec" && args[5] == "sha256sum" {
			if digest, ok := installed[args[6]]; ok {
				return host.Result{Stdout: digest + "  " + args[6]}, nil
			}
			return host.Result{}, errors.New("missing client")
		}
		if len(args) >= 9 && args[0] == "exec" && args[5] == "stat" {
			return host.Result{Stdout: "755:0:0"}, nil
		}
		return original(ctx, n, name, args)
	}
	runtime := New(runner)
	if err := runtime.SetupTrustedHost(context.Background(), dir); err == nil {
		t.Fatal("partial install accepted")
	}
	failProduct = false
	if err := runtime.SetupTrustedHost(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if err := runtime.SetupTrustedHost(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	if pushes[trustedHostClientPath] != 1 || pushes[trustedHostProductClientPath] != 2 {
		t.Fatalf("non-idempotent pushes=%v", pushes)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && (call.args[0] == "delete" || call.args[0] == "init" || call.args[0] == "stop" || call.args[0] == "start") {
			t.Fatalf("unexpected lifecycle mutation: %v", call)
		}
		if strings.Contains(strings.Join(call.args, " "), "hacoq") {
			t.Fatalf("setup depends on legacy client: %v", call)
		}
	}
}

func TestHostSetupRejectsUnownedCollisionWithoutProvisioning(t *testing.T) {
	runner := trustedHostRunner("RUNNING", "unowned", nil)
	if err := New(runner).SetupTrustedHost(context.Background(), setupClientFixtures(t)); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("err=%v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && (call.args[0] == "file" || call.args[0] == "delete") {
			t.Fatalf("unowned mutation: %v", call)
		}
	}
}

func TestHostSetupRefusesUnexpectedClientMode(t *testing.T) {
	runner := trustedHostRunner("RUNNING", trustedHostRoleValue, nil)
	original := runner.run
	runner.run = func(ctx context.Context, n int, name string, args []string) (host.Result, error) {
		if isConfigGet(args, trustedHostClientModeEnvKey) {
			return host.Result{Stdout: "local"}, nil
		}
		return original(ctx, n, name, args)
	}
	if err := New(runner).SetupTrustedHost(context.Background(), setupClientFixtures(t)); !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error=%v", err)
	}
	for _, call := range runner.calls {
		if len(call.args) > 0 && call.args[0] == "file" {
			t.Fatalf("provisioned after client mode drift: %v", call)
		}
	}
}
