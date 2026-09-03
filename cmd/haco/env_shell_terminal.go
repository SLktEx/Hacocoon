package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/term"
)

type environmentShellTerminalPreparer func(io.Reader) (func() error, error)

func prepareInteractiveTerminal(stdin io.Reader) (func() error, error) {
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

func environmentClientShellWithTerminal(
	ctx context.Context,
	client environmentControllerClient,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	prepareTerminal environmentShellTerminalPreparer,
) (retErr error) {
	if len(args) != 1 {
		return fmt.Errorf("usage: haco env shell <environment>: %w", core.ErrInvalidArgument)
	}
	if prepareTerminal == nil {
		return core.ErrInvalidArgument
	}

	stream, err := client.OpenEnvironmentShell(ctx, args[0])
	if err != nil {
		return err
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

	inputDone := make(chan error, 1)
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
