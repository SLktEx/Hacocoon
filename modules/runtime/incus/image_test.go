package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
)

type runnerCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls []runnerCall
	run   func(string, []string) (host.Result, error)
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	copied := append([]string(nil), args...)
	f.calls = append(f.calls, runnerCall{name: name, args: copied})
	if f.run == nil {
		return host.Result{}, nil
	}
	return f.run(name, copied)
}

func TestEnsureBaseImageReusesExistingAlias(t *testing.T) {
	runner := &fakeRunner{}
	runtime := New(runner)

	if err := runtime.ensureBaseImage(context.Background(), "pool"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("existing image should need one probe, calls=%v", runner.calls)
	}
	assertCall(t, runner.calls[0], "incus", "image", "show", preparedImageAlias, "--project", defaultProject)
}

func TestEnsureBaseImageBuildsPinnedVerifiedImage(t *testing.T) {
	runner := &fakeRunner{}
	runner.run = func(_ string, args []string) (host.Result, error) {
		if hasPrefix(args, "image", "show") {
			return host.Result{}, errors.New("not found")
		}
		return host.Result{}, nil
	}
	runtime := New(runner)

	if err := runtime.ensureBaseImage(context.Background(), "haco-pool"); err != nil {
		t.Fatal(err)
	}

	launch := findCall(t, runner.calls, "launch")
	if !containsSequence(launch.args, defaultSourceImage, imageBuilderName) || !containsSequence(launch.args, "-c", "security.nesting=true") || !containsSequence(launch.args, "--storage", "haco-pool") {
		t.Fatalf("unexpected builder launch: %v", launch.args)
	}

	execCall := findCall(t, runner.calls, "exec")
	script := execCall.args[len(execCall.args)-1]
	for _, want := range []string{
		"apt-get install -y --no-install-recommends ca-certificates curl tar containerd containernetworking-plugins",
		"nerdctl-" + nerdctlVersion + "-linux-${nerdctl_arch}.tar.gz",
		"sha256sum -c -",
		"systemctl is-active --quiet containerd",
		"nerdctl info >/dev/null",
		"nerdctl run --rm " + nestedSmokeImage + " true",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("provision script missing %q", want)
		}
	}

	publish := findCall(t, runner.calls, "publish")
	if !containsSequence(publish.args, "--alias", preparedImageAlias, "--reuse") {
		t.Fatalf("unexpected publish call: %v", publish.args)
	}

	deletes := countCalls(runner.calls, "delete")
	if deletes != 2 {
		t.Fatalf("expected stale-builder cleanup and deferred cleanup, got %d: %v", deletes, runner.calls)
	}
}

func TestEnsureBaseImageCleansBuilderAfterProvisionFailure(t *testing.T) {
	provisionErr := errors.New("apt failed")
	runner := &fakeRunner{}
	runner.run = func(_ string, args []string) (host.Result, error) {
		switch {
		case hasPrefix(args, "image", "show"):
			return host.Result{}, errors.New("not found")
		case hasPrefix(args, "exec"):
			return host.Result{}, provisionErr
		default:
			return host.Result{}, nil
		}
	}
	runtime := New(runner)

	err := runtime.ensureBaseImage(context.Background(), "pool")
	if !errors.Is(err, provisionErr) {
		t.Fatalf("expected provision error, got %v", err)
	}
	if countCalls(runner.calls, "delete") != 2 {
		t.Fatalf("builder was not cleaned after failure: %v", runner.calls)
	}
	if hasCall(runner.calls, "publish") {
		t.Fatalf("failed builder must not be published: %v", runner.calls)
	}
}

func TestBaseImageProvisionScriptVerifiesDownloadBeforeInstall(t *testing.T) {
	script := baseImageProvisionScript()
	checksum := strings.Index(script, "sha256sum -c -")
	extract := strings.Index(script, "tar -C /usr/local/bin")
	if checksum < 0 || extract < 0 || checksum > extract {
		t.Fatalf("nerdctl archive must be checksum-verified before extraction")
	}
}

func assertCall(t *testing.T, got runnerCall, name string, args ...string) {
	t.Helper()
	if got.name != name || strings.Join(got.args, "\x00") != strings.Join(args, "\x00") {
		t.Fatalf("call = %s %v, want %s %v", got.name, got.args, name, args)
	}
}

func findCall(t *testing.T, calls []runnerCall, subcommand string) runnerCall {
	t.Helper()
	for _, call := range calls {
		if len(call.args) > 0 && call.args[0] == subcommand {
			return call
		}
	}
	t.Fatalf("missing incus %s call: %v", subcommand, calls)
	return runnerCall{}
}

func hasCall(calls []runnerCall, subcommand string) bool {
	for _, call := range calls {
		if len(call.args) > 0 && call.args[0] == subcommand {
			return true
		}
	}
	return false
}

func countCalls(calls []runnerCall, subcommand string) int {
	count := 0
	for _, call := range calls {
		if len(call.args) > 0 && call.args[0] == subcommand {
			count++
		}
	}
	return count
}

func hasPrefix(args []string, want ...string) bool {
	if len(args) < len(want) {
		return false
	}
	for i := range want {
		if args[i] != want[i] {
			return false
		}
	}
	return true
}

func containsSequence(args []string, want ...string) bool {
	for i := 0; i+len(want) <= len(args); i++ {
		if hasPrefix(args[i:], want...) {
			return true
		}
	}
	return false
}
