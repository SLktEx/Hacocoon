package incus

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestSSHPortProbeUsesLoopbackAndReleasesListener(t *testing.T) {
	port, err := chooseLoopbackPort(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		t.Fatal("probe retained port:", err)
	}
	defer listener.Close()
}

func TestSSHPortSelectionRejectsInvalidAndCanceledRequests(t *testing.T) {
	for _, port := range []int{-1, 65536} {
		if _, err := chooseLoopbackPort(context.Background(), port); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := chooseLoopbackPort(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got, err := chooseLoopbackPort(context.Background(), 2222); err != nil || got != 2222 {
		t.Fatalf("%d: %v", got, err)
	}
}
