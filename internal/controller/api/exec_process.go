package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const MethodExecStream = "environment.process"

type ExecStreamRequest struct {
	Environment string              `json:"environment"`
	Process     core.ProcessRequest `json:"process"`
	Terminal    TerminalMetadata    `json:"terminal,omitempty"`
}

type execStreamService interface {
	ExecStream(context.Context, string, core.ProcessRequest, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error)
}

func RegisterExec(server *control.Server, process execStreamService) error {
	if server == nil || process == nil {
		return core.ErrInvalidArgument
	}
	return server.RegisterStream(MethodExecStream, func(ctx context.Context, payload json.RawMessage) (control.Stream, error) {
		if len(payload) > 64<<10 {
			return nil, control.NewStatusError("invalid_argument", "process request exceeds size limit")
		}
		var request ExecStreamRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			return nil, control.NewStatusError("invalid_argument", "invalid process request")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return nil, control.NewStatusError("invalid_argument", "trailing process request")
		}
		if err := core.ValidateProcessRequest(request.Process); err != nil {
			return nil, translateError(err)
		}
		if err := core.ValidateEnvironmentName(request.Environment); err != nil {
			return nil, translateError(err)
		}
		metadata, err := validateTerminalMetadata(request.Terminal)
		if err != nil {
			return nil, err
		}
		if request.Process.TTY {
			if metadata.Columns == 0 || metadata.Rows == 0 {
				return nil, control.NewStatusError("invalid_argument", "TTY requires terminal dimensions")
			}
			ctx = shellTerminalContext(ctx, metadata)
		} else if request.Terminal != (TerminalMetadata{}) {
			return nil, control.NewStatusError("invalid_argument", "terminal metadata requires TTY")
		}
		return func(runCtx context.Context, conn net.Conn) error {
			if request.Process.TTY {
				runCtx = core.WithTerminalMetadata(runCtx, core.TerminalMetadataFromContext(ctx))
			}
			return control.ServeProcess(runCtx, conn, func(processCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) ([]byte, error) {
				result, err := process.ExecStream(processCtx, request.Environment, request.Process, stdin, stdout, stderr)
				return json.Marshal(execResponse{Result: result, Error: statusFromError(err)})
			})
		}, nil
	})
}
