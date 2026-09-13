package incus

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

const testHostPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAABAgMEBQYHCAkKCwwNDg8QERITFBUWFxgZGhscHR4f"

func TestManagedSSHProvisionDoesNotInstallPackages(t *testing.T) {
	for _, forbidden := range []string{"apt-get", "dnf ", "apk ", "pacman "} {
		if strings.Contains(managedSSHProvisionScript, forbidden) {
			t.Fatalf("runtime SSH provisioning still installs packages with %q", forbidden)
		}
	}
	for _, required := range []string{"command -v sshd", "systemctl enable --now \"$SSH_SERVICE\""} {
		if !strings.Contains(managedSSHProvisionScript, required) {
			t.Fatalf("runtime SSH provisioning is missing %q", required)
		}
	}
}

func TestPrepareSSHAccessReportsMissingBaseCapability(t *testing.T) {
	r := &fakeRunner{run: func(_ context.Context, n int, _ string, _ []string) (host.Result, error) {
		if n == 1 {
			return host.Result{ExitCode: 127}, errors.New("guest exit 127")
		}
		return host.Result{}, nil
	}}
	_, err := New(r).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: testHostPublicKey})
	if !errors.Is(err, core.ErrUnsupported) || !strings.Contains(err.Error(), "does not provide sshd") {
		t.Fatalf("error = %v", err)
	}
	if len(r.calls) != 5 {
		t.Fatalf("calls = %#v", r.calls)
	}
	if r.calls[2].args[1] != "set" || r.calls[3].args[0] != "exec" || r.calls[4].args[1] != "unset" {
		t.Fatalf("unsafe cleanup ordering: %#v", r.calls)
	}
}

func TestSSHGrantIsDurableBeforeGuestMutationAndHasNoPort(t *testing.T) {
	r := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if args[len(args)-1] == "/etc/ssh/ssh_host_ed25519_key.pub" {
			return host.Result{Stdout: testHostPublicKey}, nil
		}
		return host.Result{}, nil
	}}
	c, err := New(r).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: testHostPublicKey})
	if err != nil || !validSSHGrantID(c.ID) || c.Host != "" || c.Port != 0 || c.TargetPort != 22 || c.HostPublicKey != testHostPublicKey {
		t.Fatalf("%+v %v", c, err)
	}
	if len(r.calls) != 4 || r.calls[0].args[1] != "set" || !strings.HasSuffix(r.calls[0].args[3], "=pending") {
		t.Fatalf("missing write-ahead grant: %+v", r.calls)
	}
	if r.calls[1].args[len(r.calls[1].args)-2] != "haco:"+c.ID {
		t.Fatal("key not grant scoped")
	}
	for _, call := range r.calls {
		for _, arg := range call.args {
			if arg == "device" || strings.Contains(arg, "listen=tcp:") {
				t.Fatal("SSH created a Host proxy")
			}
		}
	}
}

func TestSSHGrantReservationFailureDoesNotInstallKey(t *testing.T) {
	failure := errors.New("persist failed")
	r := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) { return host.Result{}, failure }}
	_, err := New(r).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: testHostPublicKey})
	if !errors.Is(err, failure) || len(r.calls) != 1 {
		t.Fatalf("%v %+v", err, r.calls)
	}
}

func TestSSHProvisionFailureRevokesKeyBeforeRemovingGrant(t *testing.T) {
	failure := errors.New("provision failed")
	r := &fakeRunner{run: func(_ context.Context, n int, _ string, _ []string) (host.Result, error) {
		if n == 1 {
			return host.Result{}, failure
		}
		return host.Result{}, nil
	}}
	_, err := New(r).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: testHostPublicKey})
	if !errors.Is(err, failure) || len(r.calls) != 5 {
		t.Fatalf("%v %+v", err, r.calls)
	}
	if r.calls[2].args[1] != "set" || r.calls[3].args[0] != "exec" || r.calls[4].args[1] != "unset" {
		t.Fatal("unsafe revoke ordering")
	}
}

func TestListPortlessGrantAndUnrelatedForward(t *testing.T) {
	id := "ssh-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	grant, _ := json.Marshal(core.ClientConnection{ID: id, Kind: "ssh", TargetPort: 22, User: "root", HostPublicKey: testHostPublicKey})
	data, _ := json.Marshal(map[string]any{"config": map[string]string{"user.hacocoon." + id: string(grant)}, "devices": map[string]any{"haco-tcp-8080-3000": map[string]string{"type": "proxy", "listen": "tcp:127.0.0.1:8080", "connect": "tcp:127.0.0.1:3000"}}})
	r := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		return host.Result{Stdout: string(data)}, nil
	}}
	got, err := New(r).ListClientConnections(context.Background(), "haco-demo")
	if err != nil || len(got) != 2 || got[0].Port != 0 || got[1].Port != 8080 {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestPendingGrantFailsClosed(t *testing.T) {
	r := &fakeRunner{run: func(context.Context, int, string, []string) (host.Result, error) {
		return host.Result{Stdout: `{"config":{"user.hacocoon.ssh-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa":"pending"}}`}, nil
	}}
	if _, err := New(r).ListClientConnections(context.Background(), "haco-demo"); !errors.Is(err, core.ErrRecoveryRequired) {
		t.Fatal(err)
	}
}
