package incus

import (
	"context"
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestPrepareSSHAccessRejectsMultilineKeyAtProviderBoundary(t *testing.T) {
	runner := &fakeRunner{}
	_, err := New(runner).PrepareSSHAccess(context.Background(), "haco-demo", core.SSHAccessRequest{
		PublicKey: "ssh-ed25519 AAAA\nssh-ed25519 BBBB",
		HostPort:  2222,
	})
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("invalid key reached Incus: %#v", runner.calls)
	}
}

func TestRevokeSSHAccessRejectsForgedConnectionIDAtProviderBoundary(t *testing.T) {
	runner := &fakeRunner{}
	err := New(runner).RevokeSSHAccess(context.Background(), "haco-demo", "ssh-2222\nssh-3333")
	if !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("invalid connection id reached Incus: %#v", runner.calls)
	}
}
