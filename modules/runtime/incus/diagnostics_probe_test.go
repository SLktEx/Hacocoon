package incus

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

// Run the exact guest probe in a local shell with inert command fixtures. This
// tests shell short-circuit/pipe exit behavior without contacting a network or
// preparing a product Environment.
func TestConnectivityProbeReportsTheFirstFailedShellStage(t *testing.T) {
	grep, err := exec.LookPath("grep")
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range []int{0, 21, 22, 23} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			dns := "exit 0"
			if code == 21 {
				dns = "exit 1"
			}
			route := "echo 'default via 192.0.2.1'"
			if code == 22 {
				route = "exit 0"
			}
			https := "exit 0"
			if code == 23 {
				https = "exit 1"
			}
			write("getent", dns)
			write("ip", route)
			write("curl", https)
			if err := os.Symlink(grep, filepath.Join(dir, "grep")); err != nil {
				t.Fatal(err)
			}
			observed := -1
			runner := &fakeRunner{run: func(ctx context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[0] != "exec" {
					return diagnosticFixture(t, args), nil
				}
				command := exec.CommandContext(ctx, "/bin/sh", "-ec", args[len(args)-1])
				command.Env = []string{"PATH=" + dir}
				err := command.Run()
				observed = 0
				var exited *exec.ExitError
				if errors.As(err, &exited) {
					observed = exited.ExitCode()
				} else if err != nil {
					t.Fatal(err)
				}
				return host.Result{ExitCode: observed}, err
			}}
			report, err := New(runner).DiagnoseHost(context.Background(), diagnosticStorage)
			if err != nil || report.Validate() != nil || observed != code || report.Healthy() != (code == 0) {
				t.Fatalf("exit=%d report=%+v error=%v", observed, report, err)
			}
			if code != 0 && !strings.Contains(report.Checks[4].Summary, map[int]string{21: "DNS lookup", 22: "route is unavailable", 23: "HTTPS to github.com failed"}[code]) {
				t.Fatalf("incorrect failure stage: %+v", report.Checks[4])
			}
		})
	}
}
