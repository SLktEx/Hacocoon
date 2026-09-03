package main

import (
	"context"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/terminalbridge"
)

type environmentShellTerminalPreparer = terminalbridge.TerminalPreparer

func prepareInteractiveTerminal(stdin io.Reader) (func() error, error) {
	return terminalbridge.PrepareInteractiveTerminal(stdin)
}

func environmentClientShellWithTerminal(
	ctx context.Context,
	client environmentControllerClient,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	prepareTerminal environmentShellTerminalPreparer,
) error {
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
	return terminalbridge.BridgeWithTerminal(ctx, stream, stdin, stdout, prepareTerminal)
}
