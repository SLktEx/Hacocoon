package incus

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestProvisionTrustedHostProductClientPushesAndVerifies(t *testing.T) {
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
		switch {
		case len(args) >= 7 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "sha256sum" && args[6] == trustedHostProductClientPath:
			if !pushed {
				return host.Result{ExitCode: 1}, errors.New("missing")
			}
			return host.Result{Stdout: digest + "  " + trustedHostProductClientPath + "\n"}, nil
		case len(args) >= 9 && args[0] == "exec" && args[1] == trustedHostName && args[4] == "--" && args[5] == "stat" && args[8] == trustedHostProductClientPath:
			return host.Result{Stdout: "755:0:0\n"}, nil
		case len(args) >= 4 && args[0] == "file" && args[1] == "push" && args[3] == trustedHostName+trustedHostProductClientPath:
			pushed = true
			return host.Result{}, nil
		default:
			return original(ctx, call, name, args)
		}
	}

	if err := New(runner).ProvisionTrustedHostProductClient(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if !pushed {
		t.Fatal("product haco client was not pushed")
	}
	assertCallContaining(t, runner.calls, "incus", []string{
		"file", "push", source, trustedHostName + trustedHostProductClientPath,
		"--project", defaultProject, "--create-dirs", "--uid", "0", "--gid", "0", "--mode", "0755",
	})
}
