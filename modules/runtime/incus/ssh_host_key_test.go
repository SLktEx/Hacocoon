package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"strings"
	"testing"
)

func TestInvalidHostKeyNeverLeavesPublishedSSHConnection(t *testing.T) {
	for _, raw := range []string{"", "-----BEGIN OPENSSH PRIVATE KEY-----", testHostPublicKey + "\nHost *\n ProxyCommand evil", "ssh-ed25519 AAAA", strings.Repeat("x", 17000)} {
		for _, cleanupFails := range []bool{false, true} {
			removed := false
			runner := &fakeRunner{run: func(_ context.Context, _ int, _ string, args []string) (host.Result, error) {
				if args[len(args)-1] == "/etc/ssh/ssh_host_ed25519_key.pub" {
					return host.Result{Stdout: raw}, nil
				}
				if len(args) > 2 && args[0] == "config" && args[2] == "remove" {
					removed = true
					if cleanupFails {
						return host.Result{}, errors.New("cleanup failed")
					}
				}
				return host.Result{}, nil
			}}
			c, err := New(runner).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{PublicKey: testHostPublicKey, HostPort: 2222})
			if err == nil || c != (core.ClientConnection{}) || !removed {
				t.Fatalf("c=%+v err=%v removed=%v", c, err, removed)
			}
			if errors.Is(err, core.ErrRecoveryRequired) != cleanupFails {
				t.Fatalf("cleanup state: %v", err)
			}
			if raw != "" && strings.Contains(err.Error(), raw) {
				t.Fatal("untrusted key output leaked into error")
			}
		}
	}
}
