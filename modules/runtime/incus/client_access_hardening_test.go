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

func TestPrepareSSHAccessPreservesBoundedProvisioningStderr(t *testing.T) {
	provisionErr := errors.New("exit status 100")
	runner := &fakeRunner{run: func(_ context.Context, call int, _ string, _ []string) (host.Result, error) {
		if call == 1 {
			return host.Result{Stderr: "apt bootstrap failed\n"}, provisionErr
		}
		return host.Result{}, nil
	}}

	_, err := New(runner).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: "ssh-ed25519 AAAA", HostPort: 2222})
	if !errors.Is(err, provisionErr) {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "apt bootstrap failed") {
		t.Fatalf("diagnostic missing from error: %v", err)
	}

	oversized := strings.Repeat("x", maxSSHProvisionDiagnosticBytes+32)
	got := boundedSSHProvisionDiagnostic(oversized)
	if len(got) != maxSSHProvisionDiagnosticBytes+3 || !strings.HasPrefix(got, "...") {
		t.Fatalf("bounded diagnostic length/prefix = %d %q", len(got), got[:min(len(got), 3)])
	}
}

func TestManagedSSHProvisionPinsAPTToManagedIPv4Proxy(t *testing.T) {
	for _, want := range []string{
		`Acquire::ForceIPv4=true`,
		`Acquire::Connect::AddrConfig=false`,
		`Acquire::http::Proxy=$proxy_http`,
		`Acquire::https::Proxy=$proxy_https`,
		`ip -4 route get "$proxy_host"`,
		`"$route_attempts" -ge 60`,
		`managed SSH bootstrap timed out waiting for IPv4 route to Hacocoon egress proxy`,
		`managed SSH bootstrap requires the Hacocoon egress proxy environment`,
	} {
		if !strings.Contains(managedSSHProvisionScript, want) {
			t.Fatalf("managed SSH provisioning script missing %q", want)
		}
	}
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
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{Stdout: `{"devices":{"haco-ssh-2222":{"type":"proxy","listen":"tcp:127.0.0.1:2222","connect":"tcp:127.0.0.1:22"},"haco-tcp-8080-3000":{"type":"proxy","listen":"tcp:127.0.0.1:8080","connect":"tcp:127.0.0.1:3000"},"foreign":{"type":"proxy","listen":"tcp:0.0.0.0:9000","connect":"tcp:127.0.0.1:9000"}}}`}, nil
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
	assertRunnerCall(t, runner.calls[0], "incus", "query", "/1.0/instances/haco-demo?project="+defaultProject)
}

func TestListClientConnectionsRejectsMalformedAPIResponse(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, _ []string) (host.Result, error) {
		return host.Result{Stdout: `not-json`}, nil
	}}
	_, err := New(runner).ListClientConnections(context.Background(), "haco-demo")
	if err == nil || !strings.Contains(err.Error(), "decode Incus client devices") {
		t.Fatalf("error = %v", err)
	}
}
