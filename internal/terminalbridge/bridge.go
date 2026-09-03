package terminalbridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

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
	defer stream.Close()

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

	cancelDone := make(chan struct{})
	defer close(cancelDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = stream.Close()
		case <-cancelDone:
		}
	}()

	inputDone := make(chan error, 1)
	go func() {
		_, copyErr := io.Copy(stream, stdin)
		if closer, ok := stream.(interface{ CloseWrite() error }); ok {
			if closeErr := closer.CloseWrite(); copyErr == nil {
				copyErr = closeErr
			}
		inputDone <- copyErr
	}()

	_, outputErr := io.Copy(stdout, stream)
	_ = stream.Close()
	if outputErr != nil && !errors.Is(outputErr, net.ErrClosed) && ctx.Err() == nil {
		return outputErr
	}
	select {
	case inputErr := <-inputDone:
		if inputErr != nil && !errors.Is(inputErr, net.ErrClosed) && ctx.Err() == nil {
			return inputErr
		}
	default:
	}
	return ctx.Err()
}
