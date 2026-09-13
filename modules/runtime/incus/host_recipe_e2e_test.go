package incus

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

// Runs only inside the existing fresh, dedicated Host-area fixture. It never
// selects or deletes an installed user's Host. CLI/Windows acceptance is separate.
func prepareHostRecipeRecreation(t *testing.T, ctx context.Context, runtime *Runtime, command func(...string) string) func() {
	t.Helper()
	service := &recipes.HostService{Root: filepath.Join(t.TempDir(), "private"), Identity: runtime.TrustedHostIdentity, Execute: runtime.RunTrustedHostCustomization}
	apply := func(update recipes.Update) {
		t.Helper()
		if err := service.Apply(ctx, update); err != nil {
			t.Fatal(err)
		}
	}
	guest := func(argv ...string) string {
		t.Helper()
		return command(append([]string{"exec", trustedHostName, "--project", runtime.project, "--"}, argv...)...)
	}
	apply(recipes.Update{})
	script := "\ufeffmkdir -p \"$HOME/.local/bin\"\r\nprintf '#!/bin/sh\\necho recipe-tool-ready\\n' > \"$HOME/.local/bin/haco-recipe-test\"\r\nchmod 700 \"$HOME/.local/bin/haco-recipe-test\"\r\necho applied >> /root/haco-recipe-count\r\n"
	apply(recipes.Update{Script: &script})
	first, err := runtime.TrustedHostIdentity(ctx)
	if err != nil {
		t.Fatal(err)
	}
	verify := func() {
		t.Helper()
		if strings.TrimSpace(guest("/root/.local/bin/haco-recipe-test")) != "recipe-tool-ready" {
			t.Fatal("recipe tool unavailable")
		}
		if guest("/bin/cat", "/root/haco-recipe-count") != "applied\n" {
			t.Fatal("recipe unexpectedly replayed")
		}
	}
	apply(recipes.Update{})
	verify()
	return func() {
		t.Helper()
		// This fixture has no NIC. network-online may keep overall boot in
		// "starting" while basic.target and the service manager are usable.
		readyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := waitHostRecipeManager(readyCtx, runtime.runner, runtime.project); err != nil {
			t.Fatal(err)
		}
		second, err := runtime.TrustedHostIdentity(ctx)
		if err != nil || second == first {
			t.Fatal("fixture did not recreate Host", err)
		}
		apply(recipes.Update{})
		verify()
		apply(recipes.Update{})
		verify()
		t.Log("PASS real Host recipe auto-application after recreation and no repeat on the same incarnation")
	}
}

func waitHostRecipeManager(ctx context.Context, runner host.Runner, project string) error {
	for {
		result, err := runner.Run(ctx, "incus", "exec", trustedHostName, "--project", project, "--", "/usr/bin/systemctl", "is-active", "basic.target")
		if err == nil && result.ExitCode == 0 && !result.StdoutTruncated && strings.TrimSpace(result.Stdout) == "active" {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("recreated fixture service manager did not become ready: %w", ctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestHostRecipeReadinessDoesNotWaitForNetworkOnline(t *testing.T) {
	want := []string{"exec", trustedHostName, "--project", "fixture-project", "--", "/usr/bin/systemctl", "is-active", "basic.target"}
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" || !reflect.DeepEqual(args, want) {
			t.Fatalf("readiness depends on unrelated boot jobs: %s %v", name, args)
		}
		return host.Result{Stdout: "active\n"}, nil
	}}
	if err := waitHostRecipeManager(context.Background(), runner, "fixture-project"); err != nil {
		t.Fatal(err)
	}
}

func TestHostRecipeReadinessFailsClosed(t *testing.T) {
	for _, result := range []host.Result{{Stdout: "activating\n"}, {Stdout: "active\n", ExitCode: 1}, {Stdout: "active\n", StdoutTruncated: true}} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) { return result, nil }}
		err := waitHostRecipeManager(ctx, runner, "fixture-project")
		cancel()
		if err == nil {
			t.Fatal("unconfirmed service manager accepted")
		}
	}
}
