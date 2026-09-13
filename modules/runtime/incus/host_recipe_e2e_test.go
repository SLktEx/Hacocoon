package incus

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		// Incus start precedes guest systemd readiness. Only read-only probes repeat.
		readyCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		for {
			result, _ := runtime.runner.Run(readyCtx, "incus", "exec", trustedHostName, "--project", runtime.project, "--", "/usr/bin/systemctl", "is-system-running", "--wait")
			state := strings.TrimSpace(result.Stdout)
			if !result.StdoutTruncated && (state == "running" || state == "degraded") {
				break
			}
			select {
			case <-readyCtx.Done():
				t.Fatal("recreated fixture systemd did not become ready")
			case <-time.After(100 * time.Millisecond):
			}
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
