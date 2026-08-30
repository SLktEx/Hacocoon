package incus

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func trustedUnixSocket(t *testing.T) (string, func()) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "egress.sock")
	listener, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		listener.Close()
		t.Fatal(err)
	}
	return path, func() { _ = listener.Close() }
}

func TestEnsureEgressProxyReplacesReservedDeviceWithExactConfig(t *testing.T) {
	socket, cleanup := trustedUnixSocket(t)
	defer cleanup()

	wantValues := map[string]string{
		"type": "proxy", "bind": "instance", "listen": egressProxyListen, "connect": "unix:" + socket,
	}
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 4 && args[0] == "config" && args[1] == "device" && args[2] == "list" {
			return host.Result{Stdout: "workspace\n" + egressProxyDevice + "\n"}, nil
		}
		if len(args) >= 6 && args[0] == "config" && args[1] == "device" && args[2] == "get" {
			return host.Result{Stdout: wantValues[args[5]] + "\n"}, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).EnsureEgressProxy(context.Background(), "haco-demo", socket); err != nil {
		t.Fatal(err)
	}

	var removed, added bool
	for _, call := range runner.calls {
		joined := strings.Join(call.args, " ")
		if strings.Contains(joined, "config device remove haco-demo "+egressProxyDevice) {
			removed = true
		}
		if strings.Contains(joined, "config device add haco-demo "+egressProxyDevice+" proxy") &&
			strings.Contains(joined, "bind=instance") && strings.Contains(joined, "listen="+egressProxyListen) &&
			strings.Contains(joined, "connect=unix:"+socket) {
			added = true
		}
	}
	if !removed || !added {
		t.Fatalf("reserved device was not replaced exactly: %#v", runner.calls)
	}
}

func TestEnsureEgressProxyRejectsUnsafeSocketBeforeIncus(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("not a socket"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "egress.sock")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	err := New(runner).EnsureEgressProxy(context.Background(), "haco-demo", link)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("Incus was called for unsafe socket: %#v", runner.calls)
	}
}

func TestEnsureEgressProxyRejectsLooseSocketPermissions(t *testing.T) {
	socket, cleanup := trustedUnixSocket(t)
	defer cleanup()
	if err := os.Chmod(socket, 0o666); err != nil {
		t.Fatal(err)
	}
	err := New(&fakeRunner{}).EnsureEgressProxy(context.Background(), "haco-demo", socket)
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
}

func TestEnsureSandboxDNSDisabledMigratesOnlyEmptyConfiguration(t *testing.T) {
	getCalls := 0
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 4 && args[0] == "network" && args[1] == "get" && args[3] == "raw.dnsmasq" {
			getCalls++
			if getCalls == 1 {
				return host.Result{}, nil
			}
			return host.Result{Stdout: sandboxDNSDisabledConfig + "\n"}, nil
		}
		return host.Result{}, nil
	}}
	if err := New(runner).ensureSandboxDNSDisabled(context.Background()); err != nil {
		t.Fatal(err)
	}
	seenSet := false
	for _, call := range runner.calls {
		if strings.Contains(strings.Join(call.args, " "), "network set "+sandboxNetwork+" raw.dnsmasq="+sandboxDNSDisabledConfig) {
			seenSet = true
		}
	}
	if !seenSet {
		t.Fatalf("DNS disablement was not persisted: %#v", runner.calls)
	}
}

func TestEnsureSandboxDNSDisabledRejectsUnmanagedRawDnsmasq(t *testing.T) {
	runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
		if len(args) >= 4 && args[0] == "network" && args[1] == "get" && args[3] == "raw.dnsmasq" {
			return host.Result{Stdout: "server=8.8.8.8\n"}, nil
		}
		return host.Result{}, nil
	}}
	err := New(runner).ensureSandboxDNSDisabled(context.Background())
	if !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v, want ErrIncompatibleState", err)
	}
}
