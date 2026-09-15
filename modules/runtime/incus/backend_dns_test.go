package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"os/exec"
	"strings"
	"testing"
)

func TestBackendDNSRejectsMalformedAnswers(t *testing.T) {
	for _, raw := range []string{`["not-an-address"]`, `["fe80::1%eth0"]`, `{"answer":"10.0.0.1"}`, strings.Repeat(" ", 4097)} {
		if _, err := decodeBackendDNS([]byte(raw)); err == nil {
			t.Fatal("untrusted backend answer accepted")
		}
	}
	addresses, err := decodeBackendDNS([]byte(`["10.0.0.1","2001:db8::1"]`))
	if err != nil || len(addresses) != 2 {
		t.Fatalf("answers=%v error=%v", addresses, err)
	}
}
func TestDisabledDNSScriptSyntax(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-n")
	cmd.Stdin = strings.NewReader(environmentDNSDisable)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script: %v %s", err, output)
	}
}

const backendDNSInstance = "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type backendDNSRunner struct {
	t             *testing.T
	scenario      string
	checks, calls int
}

func (r *backendDNSRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	if name != "incus" || len(args) != 6 || args[0] != "config" || args[1] != "get" || args[4] != "--project" || args[5] != "hacocoon" {
		r.t.Fatal("unexpected authority probe", args)
	}
	if args[2] == "haco-host" && args[3] == "user.hacocoon.role" {
		return host.Result{Stdout: "trusted-host"}, nil
	}
	if args[2] != "haco-dev" || args[3] != environmentInstanceKey {
		r.t.Fatal("wrong instance probe", args)
	}
	r.checks++
	if r.scenario == "stale" || (r.scenario == "replaced" && r.checks > 1) {
		return host.Result{Stdout: "env-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}, nil
	}
	return host.Result{Stdout: backendDNSInstance}, nil
}
func (r *backendDNSRunner) RunWithInput(ctx context.Context, input []byte, name string, args ...string) (host.Result, error) {
	r.calls++
	if _, ok := ctx.Deadline(); !ok {
		r.t.Fatal("unbounded lookup")
	}
	joined := strings.Join(args, " ")
	if string(input) != `"example.test"` || strings.Contains(joined, "example.test") || !strings.Contains(joined, backendDNSLookup) || name != "incus" {
		r.t.Fatal("query escaped literal input")
	}
	if r.scenario == "failure" {
		return host.Result{Stderr: "secret-output"}, errors.New("secret-output")
	}
	return host.Result{Stdout: `["10.0.0.7"]`}, nil
}
func TestBackendResolverPinsInstanceAndUsesBoundedLiteralInput(t *testing.T) {
	for _, scenario := range []string{"ok", "stale", "replaced", "failure"} {
		t.Run(scenario, func(t *testing.T) {
			runner := &backendDNSRunner{t: t, scenario: scenario}
			runtime := New(runner)
			addresses, err := runtime.ResolveEnvironmentName(context.Background(), "haco-dev", backendDNSInstance, "example.test")
			if scenario == "ok" {
				if err != nil || len(addresses) != 1 {
					t.Fatal(addresses, err)
				}
			} else if err == nil || strings.Contains(err.Error(), "secret-output") {
				t.Fatal("failure accepted or leaked", err)
			}
			if scenario == "stale" && runner.calls != 0 {
				t.Fatal("stale instance queried backend")
			}
			if scenario == "replaced" && !errors.Is(err, core.ErrCapabilityStale) {
				t.Fatal("replacement accepted", err)
			}
		})
	}
}
