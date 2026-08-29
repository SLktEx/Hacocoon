package incus

import (
	"context"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestInspectEnvironmentMapsIncusState(t *testing.T) {
	runner := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		return host.Result{Stdout: "RUNNING\n"}, nil
	}}
	status, err := New(runner).InspectEnvironment(context.Background(), "haco-demo")
	if err != nil || status.State != core.EnvironmentRunning {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "list", "haco-demo", "--project", defaultProject, "--format", "csv", "-c", "s")
}

func TestForwardLocalPortIsLoopbackOnly(t *testing.T) {
	runner := &fakeRunner{}
	connection, err := New(runner).ForwardLocalPort(context.Background(), "haco-demo", core.LocalPortRequest{Protocol: "tcp", HostPort: 8080, TargetPort: 3000})
	if err != nil {
		t.Fatal(err)
	}
	if connection.Host != "127.0.0.1" || connection.ID != "tcp-8080-3000" {
		t.Fatalf("connection=%#v", connection)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "config", "device", "add", "haco-demo", "haco-tcp-8080-3000", "proxy", "listen=tcp:127.0.0.1:8080", "connect=tcp:127.0.0.1:3000", "--project", defaultProject)
	if strings.Contains(strings.Join(runner.calls[0].args, " "), "0.0.0.0") {
		t.Fatal("v0.3 port forward must not bind all interfaces")
	}
}

func TestPrepareSSHDelegatesToTransactionalAccessLifecycle(t *testing.T) {
	runner := &fakeRunner{}
	key := "ssh-ed25519 AAAATEST comment with spaces"
	connection, err := New(runner).PrepareSSH(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: key, HostPort: 2222})
	if err != nil {
		t.Fatal(err)
	}
	if connection.Command != "ssh -p 2222 root@127.0.0.1" || connection.User != "root" {
		t.Fatalf("connection=%#v", connection)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls=%#v", runner.calls)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "config", "device", "add", "haco-demo", "haco-ssh-2222", "proxy", "listen=tcp:127.0.0.1:2222", "connect=tcp:127.0.0.1:22", "--project", defaultProject)
	provision := runner.calls[1]
	if provision.args[len(provision.args)-2] != key || provision.args[len(provision.args)-1] != "haco:ssh-2222" {
		t.Fatalf("managed SSH argv = %#v", provision.args)
	}
}

func TestRemoveClientConnectionUsesScopedDeviceName(t *testing.T) {
	runner := &fakeRunner{}
	if err := New(runner).RemoveClientConnection(context.Background(), "haco-demo", "tcp-8080-3000"); err != nil {
		t.Fatal(err)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "config", "device", "remove", "haco-demo", "haco-tcp-8080-3000", "--project", defaultProject)
}
