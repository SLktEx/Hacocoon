package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
	"github.com/SLktEx/Hacocoon/internal/terminalbridge"
	"golang.org/x/term"
)

func (c *Client) RunStream(ctx context.Context, spec runapp.Spec, tty bool, stdin io.Reader, stdout, stderr io.Writer) (runapp.Result, error) {
	if c == nil || ctx == nil || stdin == nil || stdout == nil || stderr == nil {
		return runapp.Result{}, core.ErrInvalidArgument
	}
	request := RunStreamRequest{Spec: spec, TTY: tty}
	prepare := terminalbridge.TerminalPreparer(func(io.Reader) (func() error, error) { return nil, nil })
	if tty {
		input, ok := stdin.(interface{ Fd() uintptr })
		if !ok || !term.IsTerminal(int(input.Fd())) {
			return runapp.Result{}, core.ErrInvalidArgument
		}
		columns, rows, err := term.GetSize(int(input.Fd()))
		if err != nil || columns <= 0 || rows <= 0 {
			return runapp.Result{}, core.ErrInvalidArgument
		}
		request.Terminal = TerminalMetadata{Columns: columns, Rows: rows, Term: os.Getenv("TERM"), ColorTerm: os.Getenv("COLORTERM")}
		prepare = terminalbridge.PrepareInteractiveTerminal
	}
	conn, err := c.wire.OpenSession(ctx, MethodRunStream, request)
	if err != nil {
		return runapp.Result{}, err
	}
	defer conn.Close()
	stream, err := control.NewProcessConn(ctx, conn, stderr)
	if err != nil {
		return runapp.Result{}, err
	}
	if err := terminalbridge.BridgeWithTerminal(ctx, stream, stdin, stdout, prepare); err != nil {
		return runapp.Result{}, err
	}
	payload, err := stream.Result()
	if err != nil {
		return runapp.Result{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var response runResponse
	if decoder.Decode(&response) != nil {
		return runapp.Result{}, control.ErrProtocol
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return runapp.Result{}, control.ErrProtocol
	}
	return response.Result, responseError(response.Error)
}
