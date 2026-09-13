package incus

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestEnsureTrustedHostToolsUsesMinimalToolContract(t *testing.T) {
	calls := 0
	runner := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		calls++
		if name != "incus" {
			t.Fatalf("command=%q", name)
		}
		wantPrefix := []string{
			"exec", trustedHostName,
			"--project", defaultProject,
			"--", "env", "-i",
			"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
			"/bin/sh", "-ec",
		}
		if len(args) != len(wantPrefix)+1 || !reflect.DeepEqual(args[:len(wantPrefix)], wantPrefix) {
			t.Fatalf("args=%q", args)
		}
		script := args[len(args)-1]
		for _, required := range []string{
			"test -x /usr/bin/git && test -x /usr/bin/gh",
			"apt-get install -y --no-install-recommends git gh",
			"test -x /usr/bin/git",
			"test -x /usr/bin/gh",
		} {
			if !strings.Contains(script, required) {
				t.Fatalf("missing %q in script %q", required, script)
			}
		}
		for _, forbidden := range []string{"gh auth", "GITHUB_TOKEN", "GH_TOKEN"} {
			if strings.Contains(script, forbidden) {
				t.Fatalf("standard provisioning must not configure credentials: %q", script)
			}
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureTrustedHostTools(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestEnsureTrustedHostToolsPropagatesProvisionFailure(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		return host.Result{}, errors.New("package failure")
	}}
	err := New(runner).ensureTrustedHostTools(context.Background())
	if err == nil || !strings.Contains(err.Error(), "ensure trusted host standard tools") {
		t.Fatalf("error=%v", err)
	}
}
