package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

func TestDurableLoopbackForwardCreateListRemove(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("durable Kubernetes port-forward state currently targets the Ubuntu/Linux haco-host")
	}
	root := t.TempDir()
	t.Setenv("HACO_ROOT", root)
	kubectl := installFakePortForwardKubectl(t)
	provider := newOwnedClientProvider(t, kubectl, nil)
	port := reserveLoopbackPort(t)

	connection, err := provider.ForwardLocalPort(context.Background(), "haco-demo", core.LocalPortRequest{
		Protocol:   "tcp",
		HostPort:   port,
		TargetPort: 8080,
	})
	if err != nil {
		t.Fatal(err)
	}
	if connection.Host != "127.0.0.1" || connection.Port != port || connection.TargetPort != 8080 || connection.Kind != "tcp" {
		t.Fatalf("connection = %#v", connection)
	}
	assertLoopbackAccepts(t, port)

	connections, err := provider.ListClientConnections(context.Background(), "haco-demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || !reflect.DeepEqual(connections[0], connection) {
		t.Fatalf("connections = %#v, want %#v", connections, connection)
	}

	// Re-create the provider to prove the connection is not process-local Go state.
	provider = newOwnedClientProvider(t, kubectl, nil)
	connections, err = provider.ListClientConnections(context.Background(), "haco-demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 1 || connections[0].ID != connection.ID {
		t.Fatalf("reconciled connections = %#v", connections)
	}

	if err := provider.RemoveClientConnection(context.Background(), "haco-demo", connection.ID); err != nil {
		t.Fatal(err)
	}
	waitLoopbackClosed(t, port)
	connections, err = provider.ListClientConnections(context.Background(), "haco-demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections) != 0 {
		t.Fatalf("connections after removal = %#v", connections)
	}
}

func TestSSHAccessUsesSameDurableLoopbackForward(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("durable Kubernetes port-forward state currently targets Linux")
	}
	t.Setenv("HACO_ROOT", t.TempDir())
	kubectl := installFakePortForwardKubectl(t)
	var execCommands [][]string
	provider := newOwnedClientProvider(t, kubectl, func(args []string) (host.Result, error) {
		if len(args) >= 6 && args[0] == "-n" && args[2] == "exec" && args[4] == "--" {
			execCommands = append(execCommands, append([]string(nil), args[5:]...))
			return host.Result{}, nil
		}
		return host.Result{}, fmt.Errorf("unexpected kubectl args: %v", args)
	})
	port := reserveLoopbackPort(t)
	connection, err := provider.PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{
		PublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITest parity@example",
		HostPort:  port,
	})
	if err != nil {
		t.Fatal(err)
	}
	if connection.Kind != "ssh" || connection.User != "root" || connection.TargetPort != 22 || connection.Host != "127.0.0.1" {
		t.Fatalf("SSH connection = %#v", connection)
	}
	if !strings.Contains(connection.Command, strconv.Itoa(port)) {
		t.Fatalf("SSH command = %q", connection.Command)
	}
	if len(execCommands) != 1 || len(execCommands[0]) < 4 || execCommands[0][0] != "sh" {
		t.Fatalf("SSH provision execs = %#v", execCommands)
	}

	if err := provider.RevokeSSHAccess(context.Background(), "haco-demo", connection.ID); err != nil {
		t.Fatal(err)
	}
	if len(execCommands) != 2 {
		t.Fatalf("SSH exec count = %d, want provision + revoke", len(execCommands))
	}
	waitLoopbackClosed(t, port)
}

func TestForwardStatePIDMismatchFailsClosed(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/proc identity test")
	}
	root := t.TempDir()
	t.Setenv("HACO_ROOT", root)
	provider := newOwnedClientProvider(t, "kubectl", nil)
	dir, err := kubeForwardStateDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateStateDir(dir); err != nil {
		t.Fatal(err)
	}
	record := forwardRecord{
		Version:        forwardStateVersion,
		EnvironmentRef: "haco-demo",
		ID:             "tcp-20000-8080",
		Kind:           "tcp",
		HostPort:       20000,
		TargetPort:     8080,
		Token:          "definitely-not-this-process",
		PID:            os.Getpid(),
		ProcStartTicks: mustProcStartTicks(t, os.Getpid()),
		State:          "active",
	}
	if err := createForwardRecord(forwardRecordPath(dir, record.EnvironmentRef, record.ID), record); err != nil {
		t.Fatal(err)
	}
	_, err = provider.ListClientConnections(context.Background(), "haco-demo")
	if !errors.Is(err, core.ErrRecoveryRequired) || !errors.Is(err, core.ErrIncompatibleState) {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(forwardRecordPath(dir, record.EnvironmentRef, record.ID)); statErr != nil {
		t.Fatalf("ambiguous state was silently removed: %v", statErr)
	}
}

func TestForwardProcessEnvironmentDoesNotInheritGitCredentials(t *testing.T) {
	t.Setenv("GH_TOKEN", "must-not-leak")
	t.Setenv("GITHUB_TOKEN", "must-not-leak-either")
	t.Setenv("GIT_ASKPASS", "/tmp/evil")
	t.Setenv("KUBECONFIG", "/trusted/kubeconfig")
	environment := cleanPortForwardEnvironment("token")
	joined := strings.Join(environment, "\n")
	for _, forbidden := range []string{"GH_TOKEN=", "GITHUB_TOKEN=", "GIT_ASKPASS="} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("ambient credential variable leaked into port-forward environment: %s", forbidden)
		}
	}
	if !strings.Contains(joined, "KUBECONFIG=/trusted/kubeconfig") {
		t.Fatalf("trusted Kubernetes authority missing from isolated kubectl environment: %q", joined)
	}
}

func newOwnedClientProvider(t *testing.T, kubectl string, extra func([]string) (host.Result, error)) *Provider {
	t.Helper()
	namespace := namespaceState{}
	namespace.Metadata.Name = "haco-demo"
	namespace.Metadata.Labels = managedLabels("demo")	namespaceJSON, err := json.Marshal(namespace)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{run: func(_ context.Context, _ string, args []string) (host.Result, error) {
		if reflect.DeepEqual(args, []string{"get", "namespace", "haco-demo", "--ignore-not-found", "-o", "json"}) {
			return host.Result{Stdout: string(namespaceJSON)}, nil
		}
		if extra != nil {
			return extra(args)
		}
		return host.Result{}, fmt.Errorf("unexpected kubectl args: %v", args)
	}}
	provider, err := New(runner, Config{Image: "example.invalid/hacocoon/systemd:26.04", Kubectl: kubectl})
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func installFakePortForwardKubectl(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	name := "kubectl-haco-test"
	path := filepath.Join(dir, name)
	script := `#!/usr/bin/env python3
import signal
import socket
import sys

mapping = sys.argv[-1]
host_port = int(mapping.split(':', 1)[0])
sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
sock.bind(('127.0.0.1', host_port))
sock.listen(8)

def stop(signum, frame):
    sock.close()
    sys.exit(0)

signal.signal(signal.SIGTERM, stop)
signal.signal(signal.SIGINT, stop)
while True:
    try:
        conn, _ = sock.accept()
        conn.close()
    except OSError:
        break
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+oldPath)
	return name
}

func reserveLoopbackPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func assertLoopbackAccepts(t *testing.T, port int) {
	t.Helper()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), time.Second)
	if err != nil {
		t.Fatalf("loopback forward not listening: %v", err)
	}
	_ = conn.Close()
}

func waitLoopbackClosed(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 50*time.Millisecond)
		if err != nil {
			return
		}
		_ = conn.Close()
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("loopback port %d remained open", port)
}

func mustProcStartTicks(t *testing.T, pid int) uint64 {
	t.Helper()
	ticks, err := procStartTicks(pid)
	if err != nil {
		t.Fatal(err)
	}
	return ticks
}
