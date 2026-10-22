package terminalbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/term"
)

// TerminalPreparer prepares a local input stream for an interactive session and
// returns a restore function when preparation changed local terminal state.
type TerminalPreparer func(io.Reader) (func() error, error)

// PrepareInteractiveTerminal puts a TTY stdin into raw mode for the lifetime of
// an interactive controller session. Non-TTY and piped input is left untouched.
func PrepareInteractiveTerminal(stdin io.Reader) (func() error, error) {
	fdReader, ok := stdin.(interface{ Fd() uintptr })
	if !ok {
		return nil, nil
	}
	fd := int(fdReader.Fd())
	if !term.IsTerminal(fd) {
		return nil, nil
	}

	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("configure local terminal: %w", err)
	}
	return func() error {
		if err := term.Restore(fd, state); err != nil {
			return fmt.Errorf("restore local terminal: %w", err)
		}
		return nil
	}, nil
}

// Bridge runs one interactive local terminal session over a controller stream.
// It owns stream closure and always restores local terminal state before return.
func Bridge(ctx context.Context, stream net.Conn, stdin io.Reader, stdout io.Writer) error {
	return BridgeWithTerminal(ctx, stream, stdin, stdout, PrepareInteractiveTerminal)
}

// BridgeWithTerminal is Bridge with an injectable terminal preparer for tests.
func BridgeWithTerminal(
	ctx context.Context,
	stream net.Conn,
	stdin io.Reader,
	stdout io.Writer,
	prepareTerminal TerminalPreparer,
) (retErr error) {
	if ctx == nil || stream == nil || stdin == nil || stdout == nil || prepareTerminal == nil {
		return errors.New("invalid interactive controller stream")
	}

	// The transport observes the caller directly and can close before its
	// cancellation reaches the signal context below. Preserve that cancellation
	// even when stream teardown or terminal restoration finishes first.
	callerCtx := ctx
	defer func() {
		if err := callerCtx.Err(); err != nil {
			retErr = err
		}
	}()

	// The caller can enter raw mode below, so process termination must first turn
	// into cooperative session cancellation. This gives the bridge a chance to
	// close the controller stream and run the terminal restore defer instead of
	// letting SIGTERM/SIGHUP terminate the process with the TTY left raw/no-echo.
	// Do not subscribe to SIGINT here: when stdin is a real raw TTY, Ctrl-C is a
	// byte for the remote PTY and must not become a local client signal.
	ctx, stopSignals := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGHUP)
	defer stopSignals()
	defer stream.Close()

	stdin, closeInput, err := ownInput(stdin)
	if err != nil {
		return err
	}
	var inputDone chan error
	var inputErr error
	inputFinished := false
	if closeInput != nil {
		// Registered before terminal restoration: stop resize observation and
		// restore the terminal while this descriptor is still open, then
		// interrupt and join the input copier before returning to its caller.
		defer func() {
			closeInput()
			if inputDone != nil {
				if !inputFinished {
					inputErr = <-inputDone
					// Closing the stream also releases a copier blocked in
					// Write. Do not turn that teardown into process failure.
					if errors.Is(inputErr, io.ErrClosedPipe) {
						inputErr = nil
					}
				}
				if retErr == nil && inputErr != nil && !errors.Is(inputErr, net.ErrClosed) && !errors.Is(inputErr, os.ErrClosed) {
					retErr = inputErr
				}
			}
		}()
	}

	restoreTerminal, err := prepareTerminal(stdin)
	if err != nil {
		return err
	}
	if restoreTerminal != nil {
		defer func() {
			if restoreErr := restoreTerminal(); restoreErr != nil && retErr == nil {
				retErr = restoreErr
			}
		}()
	}
	stopResize := watchTerminalSize(ctx, stream, stdin)
	defer func() {
		if resizeErr := stopResize(); resizeErr != nil && retErr == nil {
			retErr = resizeErr
		}
	}()

	cancelDone := make(chan struct{})
	defer close(cancelDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = stream.Close()
		case <-cancelDone:
		}
	}()

	inputDone = make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stream, stdin)
		if closer, ok := stream.(interface{ CloseWrite() error }); ok {
			if closeErr := closer.CloseWrite(); copyErr == nil {
				copyErr = closeErr
			}
		}
		inputDone <- copyErr
	}()

	_, outputErr := io.Copy(stdout, stream)
	if closeInput != nil {
		// Preserve input failures that completed before output teardown.
		select {
		case inputErr = <-inputDone:
			inputFinished = true
		default:
		}
	}
	_ = stream.Close()
	if outputErr != nil && !errors.Is(outputErr, net.ErrClosed) && ctx.Err() == nil {
		return outputErr
	}
	if closeInput == nil {
		select {
		case inputErr := <-inputDone:
			if inputErr != nil && !errors.Is(inputErr, net.ErrClosed) && ctx.Err() == nil {
				return inputErr
			}
		default:
		}
	}
	return ctx.Err()
}
