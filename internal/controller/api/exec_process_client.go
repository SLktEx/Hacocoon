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
		columns, rows, err := execTerminalSize(stdin)
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

// Fd can switch an os.File back to blocking mode. Read terminal metadata through
// its pinned raw descriptor instead, without changing caller-owned file flags.
func execTerminalSize(stdin io.Reader) (columns, rows int, err error) {
	readSize := func(fd uintptr) {
		if !term.IsTerminal(int(fd)) {
			err = core.ErrInvalidArgument
			return
		}
		columns, rows, err = term.GetSize(int(fd))
	}
	if file, ok := stdin.(*os.File); ok {
		raw, rawErr := file.SyscallConn()
		if rawErr != nil {
			return 0, 0, rawErr
		}
		if rawErr = raw.Control(readSize); rawErr != nil {
			return 0, 0, rawErr
		}
	} else if input, ok := stdin.(interface{ Fd() uintptr }); ok {
		readSize(input.Fd())
	} else {
		err = core.ErrInvalidArgument
	}
	return
}
