package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

const MethodRunStream = "run.process"

type RunStreamRequest struct {
	Spec     runapp.Spec      `json:"spec"`
	TTY      bool             `json:"tty"`
	Terminal TerminalMetadata `json:"terminal,omitempty"`
}

type runStreamService interface {
	RunStream(context.Context, runapp.Spec, bool, io.Reader, io.Writer, io.Writer) (runapp.Result, error)
}

func registerRunProcess(server *control.Server, runner runService) error {
	process, ok := runner.(runStreamService)
	if !ok {
		return nil
	}
	return server.RegisterStream(MethodRunStream, func(ctx context.Context, payload json.RawMessage) (control.Stream, error) {
		if len(payload) > 64<<10 {
			return nil, control.NewStatusError("invalid_argument", "process request exceeds size limit")
		}
		var request RunStreamRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			return nil, control.NewStatusError("invalid_argument", "invalid process request")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return nil, control.NewStatusError("invalid_argument", "trailing process request")
		}
		if err := core.ValidateProcessRequest(core.ProcessRequest{WorkingDirectory: "/workspace", Argv: request.Spec.Argv, TTY: request.TTY}); err != nil {
			return nil, translateError(err)
		}
		metadata, err := validateTerminalMetadata(request.Terminal)
		if err != nil {
			return nil, err
		}
		if request.TTY {
			if metadata.Columns == 0 || metadata.Rows == 0 {
				return nil, control.NewStatusError("invalid_argument", "TTY requires terminal dimensions")
			}
			ctx = shellTerminalContext(ctx, metadata)
		} else if request.Terminal != (TerminalMetadata{}) {
			return nil, control.NewStatusError("invalid_argument", "terminal metadata requires TTY")
		}
		return func(runCtx context.Context, conn net.Conn) error {
			if request.TTY {
				runCtx = core.WithTerminalMetadata(runCtx, core.TerminalMetadataFromContext(ctx))
			}
			return control.ServeProcess(runCtx, conn, func(processCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) ([]byte, error) {
				result, err := process.RunStream(processCtx, request.Spec, request.TTY, stdin, stdout, stderr)
				return json.Marshal(runResponse{Result: result, Error: statusFromError(err)})
			})
		}, nil
	})
}
