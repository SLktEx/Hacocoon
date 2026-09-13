//go:build linux

package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/host"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHostCustomizationVerifiesTargetAndKeepsScriptOffArguments(t *testing.T) {
	for _, owned := range []bool{false, true} {
		t.Run(map[bool]string{false: "unowned", true: "owned"}[owned], func(t *testing.T) {
			dir := t.TempDir()
			argsFile, inputFile := filepath.Join(dir, "args"), filepath.Join(dir, "input")
			t.Setenv("HACO_TEST_CAPTURE_ARGS", argsFile)
			t.Setenv("HACO_TEST_CAPTURE_INPUT", inputFile)
			t.Setenv("PATH", dir)
			body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$HACO_TEST_CAPTURE_ARGS\"\n/bin/cat > \"$HACO_TEST_CAPTURE_INPUT\"\nprintf 'synthetic-script-secret\\n' >&2\nexit 0\n"
			mock := filepath.Join(dir, "incus")
			if err := os.WriteFile(mock, []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
				want := []string{"config", "get", "haco-host", "user.hacocoon.role", "--project", "hacocoon"}
				if name != "incus" || !reflect.DeepEqual(args, want) {
					t.Fatalf("ownership check: %s %v", name, args)
				}
				marker := "unowned"
				if owned {
					marker = "trusted-host"
				}
				return host.Result{Stdout: marker}, nil
			}}
			runtime := New(runner)
			script := []byte("echo synthetic-script-secret\n")
			result, err := runtime.RunTrustedHostCustomization(context.Background(), script)
			if !owned {
				if err == nil {
					t.Fatal("unowned Host accepted")
				}
				if _, err := os.Stat(inputFile); !os.IsNotExist(err) {
					t.Fatal("unowned Host executed")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.ExitCode != 0 || result.Stderr != "synthetic-script-secret\n" {
				t.Fatalf("lost output: %+v", result)
			}
			args, _ := os.ReadFile(argsFile)
			input, _ := os.ReadFile(inputFile)
			if string(args) != "exec\nhaco-host\n--project\nhacocoon\n--cwd\n/root\n--\n/usr/bin/systemd-run\n--unit=hacocoon-user-setup\n--collect\n--wait\n--pipe\n--service-type=exec\n--working-directory=/root\n--setenv=HOME=/root\n--property=KillMode=control-group\n--property=TimeoutStopSec=5s\n--property=RuntimeMaxSec=840s\n/bin/bash\n-se\n" {
				t.Fatalf("target arguments %q", args)
			}
			if string(input) != string(script) || strings.Contains(string(args), "synthetic-script-secret") {
				t.Fatal("script was not isolated to stdin")
			}
			if err := os.WriteFile(mock, []byte(strings.Replace(body, "exit 0", "exit 7", 1)), 0700); err != nil {
				t.Fatal(err)
			}
			result, err = runtime.RunTrustedHostCustomization(context.Background(), script)
			if result.ExitCode != 7 || result.Stderr != "synthetic-script-secret\n" {
				t.Fatal("lost failing result")
			}
			if err == nil || strings.Contains(err.Error(), "synthetic-script-secret") {
				t.Fatalf("unsafe or missing failure: %v", err)
			}
		})
	}
}

func TestHostCustomizationBoundsBothOutputStreams(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	body := "#!/bin/sh\n/usr/bin/head -c 70000 /dev/zero\n/usr/bin/head -c 70000 /dev/zero >&2\nexit 23\n"
	if err := os.WriteFile(filepath.Join(dir, "incus"), []byte(body), 0700); err != nil {
		t.Fatal(err)
	}
	runtime := New(&fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		return host.Result{Stdout: "trusted-host"}, nil
	}})
	result, err := runtime.RunTrustedHostCustomization(context.Background(), []byte("true"))
	if err == nil || result.ExitCode != 23 || !result.StdoutTruncated || !result.StderrTruncated || result.StdoutBytes != 70000 || result.StderrBytes != 70000 || len(result.Stdout) > 66000 || len(result.Stderr) > 66000 {
		t.Fatal("unbounded or lost result", err)
	}
}

func TestHostIdentityRejectsMalformedAndTruncatedProviderResponses(t *testing.T) {
	for _, output := range []host.Result{{Stdout: ""}, {Stdout: "../../another-host"}, {Stdout: "12345678-1234-1234-1234-123456789abc", StdoutTruncated: true}, {Stdout: "12345678-1234-1234-1234-123456789abc", ExitCode: 1}} {
		runtime := New(&fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
			if args[3] == trustedHostRoleKey {
				return host.Result{Stdout: trustedHostRoleValue}, nil
			}
			return output, nil
		}})
		if _, err := runtime.TrustedHostIdentity(context.Background()); err == nil {
			t.Fatal("malformed incarnation accepted")
		}
	}
}
