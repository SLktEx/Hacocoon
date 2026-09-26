package controlapi

import (
	"context"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/terminalbridge"
	"golang.org/x/term"
)

func (c *Client) ExecStream(ctx context.Context, environment string, process core.ProcessRequest, stdin io.Reader, stdout, stderr io.Writer) (core.ExecutionResult, error) {
	if c == nil || ctx == nil || stdin == nil || stdout == nil || stderr == nil {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	request := ExecStreamRequest{Environment: environment, Process: process}
	prepare := terminalbridge.TerminalPreparer(func(io.Reader) (func() error, error) { return nil, nil })
	if process.TTY {
		input, ok := stdin.(interface{ Fd() uintptr })
		if !ok || !term.IsTerminal(int(input.Fd())) {
			return core.ExecutionResult{}, core.ErrInvalidArgument
		}
		columns, rows, err := term.GetSize(int(input.Fd()))
		if err != nil || columns <= 0 || rows <= 0 {
			return core.ExecutionResult{}, core.ErrInvalidArgument
		}
		request.Terminal = TerminalMetadata{Columns: columns, Rows: rows, Term: os.Getenv("TERM"), ColorTerm: os.Getenv("COLORTERM")}
		prepare = terminalbridge.PrepareInteractiveTerminal
	}
	conn, err := c.wire.OpenSession(ctx, MethodExecStream, request)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	defer func() { _ = conn.Close() }()
	stream, err := control.NewProcessConn(ctx, conn, stderr)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if err := terminalbridge.BridgeWithTerminal(ctx, stream, stdin, stdout, prepare); err != nil {
		return core.ExecutionResult{}, err
	}
	payload, err := stream.Result()
	if err != nil {
		return core.ExecutionResult{}, err
	}
	return decodeExecResult(payload)
}
