//go:build !windows

package terminalbridge

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

type terminalResizer interface {
	ResizeTerminal(context.Context, int, int) error
}

func startTerminalResizeForwarding(ctx context.Context, stream net.Conn, stdin io.Reader) (<-chan error, func()) {
	resizer, ok := stream.(terminalResizer)
	if !ok {
		return nil, func() {}
	}
	fdReader, ok := stdin.(interface{ Fd() uintptr })
	if !ok {
		return nil, func() {}
	}
	fd := int(fdReader.Fd())
	if !term.IsTerminal(fd) {
		return nil, func() {}
	}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGWINCH)
	done := make(chan struct{})
	errs := make(chan error, 1)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-signals:
				columns, rows, err := term.GetSize(fd)
				if err != nil {
					reportResizeError(errs, fmt.Errorf("read local terminal size: %w", err))
					_ = stream.Close()
					return
				}
				if columns <= 0 || rows <= 0 {
					continue
				}
				if err := resizer.ResizeTerminal(ctx, columns, rows); err != nil {
					reportResizeError(errs, fmt.Errorf("propagate terminal resize: %w", err))
					_ = stream.Close()
					return
				}
			}
		}
	}()

	var stopped bool
	return errs, func() {
		if stopped {
			return
		}
		stopped = true
		signal.Stop(signals)
		close(done)
	}
}

func reportResizeError(ch chan<- error, err error) {
	select {
	case ch <- err:
	default:
	}
}
