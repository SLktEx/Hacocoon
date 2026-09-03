package main

import (
	"context"
	"io"
	"net"

	"github.com/SLktEx/Hacocoon/internal/terminalbridge"
)

// bridgeControllerStream bridges one interactive controller-backed shell using
// the shared terminal lifecycle used by every Hacocoon shell entry point.
func bridgeControllerStream(ctx context.Context, stream net.Conn, stdin io.Reader, stdout io.Writer) error {
	return terminalbridge.Bridge(ctx, stream, stdin, stdout)
}
