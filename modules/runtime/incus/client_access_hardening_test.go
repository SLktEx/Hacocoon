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

func TestPrepareSSHAccessReservesProxyBeforeMutatingKeys(t *testing.T) {
	proxyErr := errors.New("port already in use")
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
		if call == 0 {
			return host.Result{}, proxyErr
		}
		return host.Result{}, nil
	}}

	_, err := New(runner).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: "ssh-ed25519 AAAA", HostPort: 2222})
	if !errors.Is(err, proxyErr) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("failed proxy reservation mutated environment: %#v", runner.calls)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "config", "device", "add", "haco-demo", "haco-ssh-2222", "proxy", "listen=tcp:127.0.0.1:2222", "connect=tcp:127.0.0.1:22", "--project", defaultProject)
}

func TestPrepareSSHAccessRollsBackProxyWhenProvisioningFails(t *testing.T) {
	provisionErr := errors.New("sshd setup failed")
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
		if call == 1 {
			return host.Result{}, provisionErr
		}
		return host.Result{}, nil
	}}

	_, err := New(runner).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: "ssh-ed25519 AAAA", HostPort: 2222})
	if !errors.Is(err, provisionErr) {
		t.Fatalf("error = %v", err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	assertRunnerCall(t, runner.calls[2], "incus", "config", "device", "remove", "haco-demo", "haco-ssh-2222", "--project", defaultProject)
}

func TestPrepareSSHAccessRequiresSSHReadyBaseWithoutPackageInstall(t *testing.T) {
	provisionErr := errors.New("exit status 127")
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, args []string) (host.Result, error) {
		if call == 1 {
			joined := strings.Join(args, " ")
			if strings.Contains(joined, "apt-get") {
				t.Fatalf("SSH provisioning must not install packages through sandbox egress: %q", joined)
			}
			return host.Result{ExitCode: 127}, provisionErr
		}
		return host.Result{}, nil
	}}

	_, err := New(runner).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: "ssh-ed25519 AAAA", HostPort: 2222})
	if !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("error = %v want ErrUnsupported", err)
	}
	if len(runner.calls) != 3 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	assertRunnerCall(t, runner.calls[2], "incus", "config", "device", "remove", "haco-demo", "haco-ssh-2222", "--project", defaultProject)
}

func TestPrepareSSHAccessUsesConnectionScopedManagedKey(t *testing.T) {
	runner := &fakeRunner{}
	key := "ssh-ed25519 AAAATEST"
	connection, err := New(runner).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: key, HostPort: 2222})
	if err != nil {
		t.Fatal(err)
	}
	if connection.ID != "ssh-2222" || connection.Command != "ssh -p 2222 root@127.0.0.1" {
		t.Fatalf("connection = %#v", connection)
	}
	assertRunnerCall(t, runner.calls[0], "incus", "config", "device", "add", "haco-demo", "haco-ssh-2222", "proxy", "listen=tcp:127.0.0.1:2222", "connect=tcp:127.0.0.1:22", "--project", defaultProject)
	provision := runner.calls[1]
	if provision.args[len(provision.args)-2] != key || provision.args[len(provision.args)-1] != "haco:ssh-2222" {
		t.Fatalf("managed key argv = %#v", provision.args)
	}
}

func TestRevokeSSHAccessRemovesManagedKeyBeforeProxy(t *testing.T) {
	runner := &fakeRunner{}
	if err := New(runner).RevokeSSHAccess(context.Background(), "haco-demo", "ssh-2222"); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	if got := runner.calls[0].args[len(runner.calls[0].args)-1]; got != "haco:ssh-2222" {
		t.Fatalf("revoke marker = %q", got)
	}
	assertRunnerCall(t, runner.calls[1], "incus", "config", "device", "remove", "haco-demo", "haco-ssh-2222", "--project", defaultProject)
}

func TestListClientConnectionsReconcilesManagedProxyDevices(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
		switch call {
		case 0:
			return host.Result{Stdout: "haco-tcp-8080-3000\nforeign\nhaco-ssh-2222\n"}, nil
		case 1, 4:
			return host.Result{Stdout: "proxy\n"}, nil
		case 2:
			return host.Result{Stdout: "tcp:127.0.0.1:8080\n"}, nil
		case 3:
			return host.Result{Stdout: "tcp:127.0.0.1:3000\n"}, nil
		case 5:
			return host.Result{Stdout: "tcp:127.0.0.1:2222\n"}, nil
		case 6:
			return host.Result{Stdout: "tcp:127.0.0.1:22\n"}, nil
		default:
			t.Fatalf("unexpected runner call %d", call)
			return host.Result{}, nil
		}
	}}

	connections, err := New(runner).ListClientConnections(context.Background(), "haco-demo")
	if err != nil {
		t.Fatal(err)
	}
	want := []core.ClientConnection{
		{ID: "ssh-2222", Kind: "ssh", Host: "127.0.0.1", Port: 2222, TargetPort: 22, User: "root", Command: "ssh -p 2222 root@127.0.0.1"},
		{ID: "tcp-8080-3000", Kind: "tcp", Host: "127.0.0.1", Port: 8080, TargetPort: 3000},
	}
	if !reflect.DeepEqual(connections, want) {
		t.Fatalf("connections = %#v want %#v", connections, want)
	}

	assertRunnerCall(t, runner.calls[0], "incus", "config", "device", "list", "haco-demo", "--project", defaultProject)
	assertRunnerCall(t, runner.calls[1], "incus", "config", "device", "get", "haco-demo", "haco-tcp-8080-3000", "type", "--project", defaultProject)
	assertRunnerCall(t, runner.calls[2], "incus", "config", "device", "get", "haco-demo", "haco-tcp-8080-3000", "listen", "--project", defaultProject)
	assertRunnerCall(t, runner.calls[3], "incus", "config", "device", "get", "haco-demo", "haco-tcp-8080-3000", "connect", "--project", defaultProject)
	assertRunnerCall(t, runner.calls[4], "incus", "config", "device", "get", "haco-demo", "haco-ssh-2222", "type", "--project", defaultProject)
	assertRunnerCall(t, runner.calls[5], "incus", "config", "device", "get", "haco-demo", "haco-ssh-2222", "listen", "--project", defaultProject)
	assertRunnerCall(t, runner.calls[6], "incus", "config", "device", "get", "haco-demo", "haco-ssh-2222", "connect", "--project", defaultProject)
}
