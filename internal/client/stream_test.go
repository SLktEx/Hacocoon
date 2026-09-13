package client

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"net"
	"testing"
	"time"
)

type streamRuntimeFixture struct {
	fakeRuntime
	dialed bool
	peer   net.Conn
}

func (r *streamRuntimeFixture) DialEnvironmentNetwork(_ context.Context, ref, instance, protocol string, port int) (net.Conn, error) {
	if ref != "haco-demo" || instance != "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" || protocol != "tcp" || port != 22 {
		return nil, core.ErrInvalidArgument
	}
	r.dialed = true
	c, p := net.Pipe()
	r.peer = p
	return c, nil
}

type streamLifecycleFixture struct{ started bool }

func (l *streamLifecycleFixture) WithClientAccess(ctx context.Context, name string, target *core.StreamTarget, resume bool, authorize func(core.Environment, string) error, operation func(core.Environment, string) error) error {
	if name != "demo" || !resume {
		return core.ErrInvalidArgument
	}
	if target == nil {
		target = &core.StreamTarget{Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
	}
	env := core.Environment{Name: "demo", RuntimeRef: "haco-demo", Workspace: core.Workspace{ID: "work"}, AccessMode: core.WorkspaceReadWrite}
	if authorize != nil {
		if err := authorize(env, target.Instance); err != nil {
			return err
		}
	}
	l.started = true
	return operation(env, target.Instance)
}

func TestDisconnectClosesAllStreamsForOnlyTheRequestedGrant(t *testing.T) {
	target := core.StreamTarget{Environment: "demo", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "ssh-one"}
	r := &streamRuntimeFixture{}
	r.connections = []core.ClientConnection{{ID: "ssh-one", Kind: "ssh", TargetPort: 22}, {ID: "ssh-two", Kind: "ssh", TargetPort: 22}}
	s := NewWithLifecycle(r, fakeStore{environment: core.Environment{Name: "demo", RuntimeRef: "haco-demo"}}, &streamLifecycleFixture{})
	var revoked []net.Conn
	for i := 0; i < 2; i++ {
		conn, err := s.DialStream(context.Background(), target)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		revoked = append(revoked, r.peer)
		defer r.peer.Close()
	}
	target.Grant = "ssh-two"
	other, err := s.DialStream(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	peer := r.peer
	defer peer.Close()
	if err = s.Unforward(context.Background(), "demo", "ssh-one"); err != nil {
		t.Fatal(err)
	}
	for _, p := range revoked {
		_ = p.SetReadDeadline(time.Now().Add(time.Second))
		if _, err = p.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
			t.Fatalf("revoked stream remained open: %v", err)
		}
	}
	_ = peer.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
	if _, err = peer.Read(make([]byte, 1)); err == nil {
		t.Fatal("unexpected bytes")
	} else if timeout, ok := err.(net.Error); !ok || !timeout.Timeout() {
		t.Fatalf("unrelated grant closed: %v", err)
	}
}
func TestStreamRequiresExactLiveGrantBeforeResumeAndOnlyDialsSSHService(t *testing.T) {
	for _, kind := range []string{"valid", "revoked", "wrong-grant", "legacy-port", "wrong-service"} {
		t.Run(kind, func(t *testing.T) {
			target := core.StreamTarget{Environment: "demo", Instance: "env-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Workspace: "work", AccessMode: core.WorkspaceReadWrite, Service: "ssh", Grant: "ssh-one"}
			grant := core.ClientConnection{ID: target.Grant, Kind: "ssh", TargetPort: 22}
			r := &streamRuntimeFixture{}
			l := &streamLifecycleFixture{}
			switch kind {
			case "wrong-grant":
				grant.ID = "ssh-other"
			case "legacy-port":
				grant.Port = 2222
			case "wrong-service":
				target.Service = "arbitrary-tcp"
			}
			if kind != "revoked" {
				r.connections = []core.ClientConnection{grant}
			}
			conn, err := NewWithLifecycle(r, fakeStore{}, l).DialStream(context.Background(), target)
			if kind == "valid" {
				if err != nil || conn == nil || !r.dialed {
					t.Fatal(err)
				}
				conn.Close()
				r.peer.Close()
			} else {
				if err == nil || conn != nil || r.dialed || l.started {
					t.Fatal("rejected target resumed or dialed")
				}
				if kind != "wrong-service" && !errors.Is(err, core.ErrPolicyDenied) {
					t.Fatal(err)
				}
			}
		})
	}
}
