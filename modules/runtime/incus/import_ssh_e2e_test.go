//go:build linux

package incus

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/crypto/ssh"
)

// The private client key exists only in this process. Host identity comes from
// the trusted controller response, never from an unauthenticated network scan.
func verifyImportedSSH(t *testing.T, ctx context.Context, runtime *Runtime, root, name, native, workPath string, invoke func(...string) []byte) {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	must(err)
	signer, err := ssh.NewSignerFromKey(private)
	must(err)
	publicPath := filepath.Join(root, "client.pub")
	must(os.WriteFile(publicPath, ssh.MarshalAuthorizedKey(signer.PublicKey()), 0600))
	var connection core.ClientConnection
	must(json.Unmarshal(invoke("env", "ssh", "--key", publicPath, name), &connection))
	if connection.Kind != "ssh" || connection.Host != "127.0.0.1" || connection.Port < 1 || connection.Port > 65535 || connection.TargetPort != 22 || connection.User != "root" || connection.ID == "" {
		t.Fatal("imported SSH connection has unexpected boundary")
	}
	hostKey, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(connection.HostPublicKey))
	must(err)
	if len(options) != 0 || len(rest) != 0 || hostKey.Type() != ssh.KeyAlgoED25519 {
		t.Fatal("invalid controller SSH host identity")
	}
	address := net.JoinHostPort(connection.Host, strconv.Itoa(connection.Port))
	dialCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	socket, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", address)
	must(err)
	defer socket.Close()
	must(socket.SetDeadline(time.Now().Add(30 * time.Second)))
	config := &ssh.ClientConfig{User: connection.User, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.FixedHostKey(hostKey)}
	clientConn, channels, requests, err := ssh.NewClientConn(socket, address, config)
	must(err)
	client := ssh.NewClient(clientConn, channels, requests)
	defer client.Close()
	session, err := client.NewSession()
	must(err)
	// The path is a verified managed attachment, but still quote it as shell data.
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
	output, err := session.Output("cd -- " + quote(workPath) + " && test -d .git && cat tracked && printf '%s' continued-over-ssh > import-ssh-marker")
	_ = session.Close()
	must(err)
	if !strings.HasPrefix(string(output), "uncommitted workspace-") {
		t.Fatal("SSH did not reach imported Git Workspace")
	}
	must(client.Close())
	invoke("env", "disconnect", name, connection.ID)
	connections, err := runtime.ListClientConnections(ctx, native)
	must(err)
	for _, current := range connections {
		if current.ID == connection.ID {
			t.Fatal("revoked imported SSH connection remained")
		}
	}
	result, err := runtime.runner.Run(ctx, "incus", "exec", native, "--project", runtime.project, "--", "cat", "/root/.ssh/authorized_keys")
	must(err)
	if result.ExitCode != 0 || result.StdoutTruncated || strings.Contains(result.Stdout, "haco:"+connection.ID) {
		t.Fatal("revoked imported SSH authorization remained")
	}
	t.Log("PASS imported SSH: public CLI preparation, fresh in-memory client key, controller-pinned host key, real authenticated session, Git Workspace write and connection/key revocation")
}
