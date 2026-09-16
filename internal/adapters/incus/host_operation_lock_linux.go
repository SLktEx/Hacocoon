//go:build linux

package incus

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// An abstract Unix bind is a kernel-held cross-process lock, without writable
// lock-file paths or stale files. Nothing is accepted/read on this socket. A
// crash releases it; the durable provider journal still prevents unsafe restart.
func lockHostOperation(ctx context.Context, project string) (func(), error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	sum := sha256.Sum256([]byte(project))
	name := fmt.Sprintf("@hacocoon-host-lifecycle-%x", sum[:])
	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("Host lifecycle busy: %w: %w", core.ErrStorageBusy, err)
		}
		listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: name, Net: "unix"})
		if err == nil {
			listener.SetUnlinkOnClose(false)
			return func() { _ = listener.Close() }, nil
		}
		if !errors.Is(err, syscall.EADDRINUSE) {
			return nil, core.ErrRuntimeUnavailable
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
}
