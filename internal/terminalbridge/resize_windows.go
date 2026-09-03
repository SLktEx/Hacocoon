//go:build windows

package terminalbridge

import (
	"context"
	"io"
	"net"
)

func startTerminalResizeForwarding(context.Context, net.Conn, io.Reader) (<-chan error, func()) {
	return nil, func() {}
}
