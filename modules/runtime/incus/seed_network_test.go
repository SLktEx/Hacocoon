package incus

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestCreateToolingBuilderNetworkIsSeedScopedAndEphemeral(t *testing.T) {
	runner := &fakeRunner{}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	name, cleanup, err := provider.createToolingBuilderNetwork(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "haco-t-") || len(name) != 15 {
		t.Fatalf("network name = %q, want 15-character haco-t-* Linux interface name", name)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	wantCreate := []string{
		"network", "create", name,
		"ipv4.address=auto",
		"ipv4.nat=true",
		"ipv4.firewall=true",
		"ipv4.routing=true",
		"ipv6.address=none",
		"--project", sandboxResourceProject,
	}
	if runner.calls[0].name != "incus" || !reflect.DeepEqual(runner.calls[0].args, wantCreate) {
		t.Fatalf("create call = %#v, want args=%#v", runner.calls[0], wantCreate)
	}
	for _, arg := range runner.calls[0].args {
		if arg == sandboxNetwork || arg == sandboxEgressACL || arg == "raw.dnsmasq=port=0" {
			t.Fatalf("trusted tooling network reused sandbox transport policy: %#v", runner.calls[0])
		}
	}

	if err := cleanup(nil); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	assertRunnerCall(t, runner.calls[1], "incus", "network", "delete", name, "--project", sandboxResourceProject)
}

func TestToolingBuilderNetworkCleanupFailureRequiresRecovery(t *testing.T) {
	deleteErr := errors.New("network still in use")
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 2 && args[0] == "network" && args[1] == "delete" {
			return host.Result{ExitCode: 1, Stderr: deleteErr.Error()}, deleteErr
		}
		return host.Result{}, nil
	}}
	provider, err := NewSandboxProvider(New(runner))
	if err != nil {
		t.Fatal(err)
	}

	_, cleanup, err := provider.createToolingBuilderNetwork(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := cleanup(nil); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatalf("cleanup error = %v, want ErrRecoveryRequired", err)
	}
}
