package terminalbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"golang.org/x/term"
)

type terminalResizer interface {
	SupportsResize() bool
	Resize(context.Context, int, int) error
}

func watchTerminalSize(ctx context.Context, stream net.Conn, stdin io.Reader) func() error {
	resizer, ok := stream.(terminalResizer)
	if !ok || !resizer.SupportsResize() {
		return func() error { return nil }
	}
	input, ok := stdin.(interface{ Fd() uintptr })
	if !ok || !term.IsTerminal(int(input.Fd())) {
		return func() error { return nil }
	}
	ctx, cancel := context.WithCancel(ctx)
	changes, stopChanges := terminalSizeChanges()
	done := make(chan error, 1)
	go func() {
		var lastColumns, lastRows int
		for {
			columns, rows, err := term.GetSize(int(input.Fd()))
			if err != nil {
				done <- fmt.Errorf("read local terminal size: %w", err)
				_ = stream.Close()
				return
			}
			if columns > 0 && rows > 0 && (columns != lastColumns || rows != lastRows) {
				callCtx, stopCall := context.WithTimeout(ctx, 5*time.Second)
				err = resizer.Resize(callCtx, columns, rows)
				stopCall()
				if err != nil {
					if errors.Is(err, io.EOF) {
						done <- nil
						return
					}
					if ctx.Err() != nil {
						err = nil
					}
					done <- err
					_ = stream.Close()
					return
				}
				lastColumns, lastRows = columns, rows
			}
			select {
			case <-ctx.Done():
				done <- nil
				return
			case <-changes:
			}
		}
	}()
	return func() error {
		cancel()
		stopChanges()
		return <-done
	}
}
