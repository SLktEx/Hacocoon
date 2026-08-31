package incus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestTrustedHostOCIRunnerKeepsIncusOnPhysicalHost(t *testing.T) {
	physical := &fakeRunner{}
	runtime := New(physical)
	runner := &TrustedHostOCIRunner{runtime: runtime, physical: physical, ready: true}

	if _, err := runner.Run(context.Background(), "incus", "version"); err != nil {
		t.Fatal(err)
	}
	assertRunnerCall(t, physical.calls[0], "incus", "version")
}

func TestTrustedHostOCIRunnerRoutesNerdctlIntoTrustedHost(t *testing.T) {
	physical := &fakeRunner{}
	runtime := New(physical)
	runner := &TrustedHostOCIRunner{runtime: runtime, physical: physical, ready: true}

	if _, err := runner.Run(context.Background(), "nerdctl", "--namespace", seedHostNamespace, "image", "inspect", "busybox@sha256:deadbeef"); err != nil {
		t.Fatal(err)
	}
	assertRunnerCall(t, physical.calls[0], "incus", "exec", trustedHostName, "--project", defaultProject, "--",
		"nerdctl", "--namespace", seedHostNamespace, "image", "inspect", "busybox@sha256:deadbeef")
	for _, call := range physical.calls {
		if call.name == "nerdctl" {
			t.Fatalf("nerdctl escaped onto Physical Host: %#v", call)
		}
	}
}

func TestTrustedHostOCIRunnerRefusesUnscopedTooling(t *testing.T) {
	physical := &fakeRunner{}
	runner := &TrustedHostOCIRunner{runtime: New(physical), physical: physical, ready: true}
	_, err := runner.Run(context.Background(), "curl", "https://example.invalid")
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("error = %v", err)
	}
	if len(physical.calls) != 0 {
		t.Fatalf("unexpected Physical Host command: %#v", physical.calls)
	}
}

func TestTrustedHostOCIRunnerBridgesNamespacedSaveThroughIncusFilePull(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "images.tar")
	physical := &fakeRunner{run: func(_ context.Context, _ int, name string, args []string) (host.Result, error) {
		if name != "incus" {
			t.Fatalf("Physical Host executed %q", name)
		}
		if len(args) >= 4 && args[0] == "file" && args[1] == "pull" {
			if args[3] != output {
				t.Fatalf("pull destination = %q want %q", args[3], output)
			}
			if err := os.WriteFile(output, []byte("oci-archive"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		return host.Result{}, nil
	}}
	runtime := New(physical)
	runner := &TrustedHostOCIRunner{runtime: runtime, physical: physical, ready: true}

	_, err := runner.Run(context.Background(), "nerdctl", "--namespace", seedHostNamespace, "save", "-o", output, "busybox@sha256:deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(output); err != nil {
		t.Fatal(err)
	}

	var sawGuestSave, sawPull bool
	for _, call := range physical.calls {
		if call.name != "incus" {
			t.Fatalf("unexpected Physical Host command: %#v", call)
		}
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, " -- nerdctl --namespace "+seedHostNamespace+" save -o ") {
			sawGuestSave = true
			if strings.Contains(joined, output) {
				t.Fatalf("Physical Host path leaked into haco-host command: %s", joined)
			}
		}
		if len(call.args) >= 2 && call.args[0] == "file" && call.args[1] == "pull" {
			sawPull = true
		}
	}
	if !sawGuestSave || !sawPull {
		t.Fatalf("guest save=%v file pull=%v calls=%#v", sawGuestSave, sawPull, physical.calls)
	}
}

func TestTrustedHostOCIRunnerBridgesNamespacedLoadThroughIncusFilePush(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "images.tar")
	if err := os.WriteFile(input, []byte("oci-archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	physical := &fakeRunner{}
	runtime := New(physical)
	runner := &TrustedHostOCIRunner{runtime: runtime, physical: physical, ready: true}

	_, err := runner.Run(context.Background(), "nerdctl", "--namespace", seedHostNamespace, "load", "-i", input)
	if err != nil {
		t.Fatal(err)
	}
	var sawPush, sawGuestLoad bool
	for _, call := range physical.calls {
		if call.name != "incus" {
			t.Fatalf("unexpected Physical Host command: %#v", call)
		}
		joined := strings.Join(call.args, " ")
		if len(call.args) >= 2 && call.args[0] == "file" && call.args[1] == "push" {
			sawPush = true
			if call.args[2] != input {
				t.Fatalf("push source = %q want %q", call.args[2], input)
			}
		}
		if strings.Contains(joined, " -- nerdctl --namespace "+seedHostNamespace+" load -i ") {
			sawGuestLoad = true
			if strings.Contains(joined, input) {
				t.Fatalf("Physical Host path leaked into haco-host command: %s", joined)
			}
		}
	}
	if !sawPush || !sawGuestLoad {
		t.Fatalf("file push=%v guest load=%v calls=%#v", sawPush, sawGuestLoad, physical.calls)
	}
}

func TestCloneSandboxProviderWithRunnerDoesNotChangeEnvironmentProvider(t *testing.T) {
	physical := &fakeRunner{}
	seed := &fakeRunner{}
	provider, err := NewSandboxProvider(New(physical))
	if err != nil {
		t.Fatal(err)
	}
	clone, err := CloneSandboxProviderWithRunner(provider, seed)
	if err != nil {
		t.Fatal(err)
	}
	if provider.runner != physical {
		t.Fatal("ordinary Environment provider runner changed")
	}
	if clone.runner != seed {
		t.Fatal("Seed provider did not receive trusted runner")
	}
	if clone.Runtime.storage != provider.Runtime.storage {
		t.Fatal("Seed provider did not retain managed storage state")
	}
}

func TestTrustedHostSaveAndLoadRecognizeNamespacePrefix(t *testing.T) {
	if got, ok, err := trustedHostSaveOutput([]string{"--namespace", seedHostNamespace, "save", "-o", "/tmp/a.tar", "ref"}); err != nil || !ok || got != "/tmp/a.tar" {
		t.Fatalf("save got=%q ok=%v err=%v", got, ok, err)
	}
	if got, ok, err := trustedHostLoadInput([]string{"--namespace", seedHostNamespace, "load", "-i", "/tmp/a.tar"}); err != nil || !ok || got != "/tmp/a.tar" {
		t.Fatalf("load got=%q ok=%v err=%v", got, ok, err)
	}
}
