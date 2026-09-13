//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

type hostRecipeFixture struct{ service *recipes.HostService }

func (f hostRecipeFixture) SetupHost(ctx context.Context, update recipes.Update) error {
	return f.service.Apply(ctx, update)
}

func TestHostRecipeCLIPrivateFailureResultAndExplicitReplay(t *testing.T) {
	calls := 0
	service := &recipes.HostService{Root: filepath.Join(t.TempDir(), "private"), Identity: func(context.Context) (string, error) { return "12345678-1234-1234-1234-123456789abc", nil }, Execute: func(context.Context, []byte) (host.Result, error) {
		calls++
		return host.Result{ExitCode: 23, Stdout: "PRIVATE-STDOUT", Stderr: "PRIVATE-STDERR"}, errors.New("PRIVATE-BACKEND")
	}}
	t.Setenv("HACO_CONTROL_SOCKET", productSetupServiceServer(t, hostRecipeFixture{service}))
	path := filepath.Join(t.TempDir(), "setup with spaces.sh")
	if err := os.WriteFile(path, []byte("\ufeffexit 23\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	run := func(args ...string) int {
		stdout.Reset()
		stderr.Reset()
		return setup(context.Background(), args, &stdout, &stderr)
	}
	if code := run("--script", path); code != 1 || calls != 1 || !strings.Contains(stderr.String(), "exit_code=23") || strings.Contains(stdout.String()+stderr.String(), "PRIVATE") {
		t.Fatal(code, stdout.String(), stderr.String(), calls)
	}
	if code := run(); code != 1 || calls != 1 {
		t.Fatal("implicit retry", code, calls)
	}
	if code := run("--script-result"); code != 0 || calls != 1 || stdout.String() != "PRIVATE-STDOUT" || !strings.Contains(stderr.String(), "PRIVATE-STDERR") || strings.Contains(stderr.String(), "PRIVATE-BACKEND") {
		t.Fatal("result inspection", code, stdout.String(), stderr.String())
	}
	if code := run("--reapply-script"); code != 1 || calls != 2 {
		t.Fatal("explicit retry", code, calls)
	}
	for _, args := range [][]string{{"--reapply-script", "env"}, {"--script-result", "env"}, {"--reapply-script", "--clear-script"}, {"--script-result", "--reapply-script"}} {
		if code := run(args...); code != 2 || calls != 2 {
			t.Fatal("invalid Host options", args, code)
		}
	}
}
