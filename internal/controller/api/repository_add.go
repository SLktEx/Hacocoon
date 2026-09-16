package controlapi

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
)

type repositoryAdder interface {
	Add(context.Context, string, string) (gitrepo.Object, error)
}

type repositoryFrame struct {
	Progress string          `json:"progress,omitempty"`
	Done     bool            `json:"done,omitempty"`
	Result   *gitrepo.Object `json:"result,omitempty"`
	Code     string          `json:"code,omitempty"`
}

func registerRepositoryAdd(server *control.Server, service repositoryAdder) error {
	return server.RegisterStream(MethodRepositoryAdd, func(_ context.Context, raw json.RawMessage) (control.Stream, error) {
		var req RepositoryAddRequest
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF || !gitadapter.ValidID(req.ID) || gitadapter.ValidateRemote(req.Remote) != nil {
			return nil, control.ErrInvalidArgument
		}
		return func(ctx context.Context, conn net.Conn) error {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			// No client input is legal after the request. EOF cancels even when
			// Git produces no output. Keep the service lock until it has stopped.
			go func() { var b [1]byte; _, _ = conn.Read(b[:]); cancel() }()
			encoder := json.NewEncoder(conn)
			var writeErr error
			send := func(frame repositoryFrame) error {
				if writeErr == nil {
					writeErr = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if writeErr == nil {
						writeErr = encoder.Encode(frame)
					}
				}
				if writeErr != nil {
					cancel()
				}
				return writeErr
			}
			progress := gitadapter.NewGitDiagnostic(repositoryProgressFunc(func(line string) error {
				return send(repositoryFrame{Progress: strings.TrimSuffix(line, "\n")})
			}))
			result, err := service.Add(gitadapter.WithProgress(ctx, progress), req.ID, req.Remote)
			progress.Flush()
			code := ""
			if err != nil {
				code = "repository_failed"
				switch {
				case errors.Is(err, core.ErrRecoveryRequired):
					code = "recovery_required"
				case errors.Is(err, core.ErrAlreadyExists):
					code = "already_exists"
				}
			}
			return send(repositoryFrame{Done: true, Result: &result, Code: code})
		}, nil
	})
}

type repositoryProgressFunc func(string) error

func (f repositoryProgressFunc) Write(p []byte) (int, error) { return len(p), f(string(p)) }

// AddRepository requires an explicit final frame. A lost connection is never
// retried as a second mutation or interpreted as successful registration.
func (c *Client) AddRepository(ctx context.Context, req RepositoryAddRequest, progress io.Writer) (gitrepo.Object, error) {
	conn, err := c.wire.OpenStream(ctx, MethodRepositoryAdd, req)
	if err != nil {
		return gitrepo.Object{}, err
	}
	defer conn.Close()
	reader := bufio.NewReaderSize(conn, 32<<10)
	for {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			if ctx.Err() != nil {
				return gitrepo.Object{}, ctx.Err()
			}
			return gitrepo.Object{}, control.ErrProtocol
		}
		var frame repositoryFrame
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&frame) != nil || decoder.Decode(new(any)) != io.EOF {
			return gitrepo.Object{}, control.ErrProtocol
		}
		if frame.Done {
			if frame.Progress != "" || frame.Result == nil {
				return gitrepo.Object{}, control.ErrProtocol
			}
			switch frame.Code {
			case "":
				if frame.Result.Kind != "repo" || frame.Result.ID != req.ID || frame.Result.Remote != req.Remote || frame.Result.Branch != "" || frame.Result.State != "ready" {
					return gitrepo.Object{}, control.ErrProtocol
				}
				return *frame.Result, nil
			case "repository_failed", "already_exists", "recovery_required":
				return *frame.Result, control.NewStatusError(frame.Code, "Repository registration did not complete; inspect repo list for retained ownership before retrying")
			default:
				return gitrepo.Object{}, control.ErrProtocol
			}
		}
		if frame.Result != nil || frame.Code != "" || frame.Progress == "" || gitadapter.SafeGitLine(frame.Progress) != frame.Progress {
			return gitrepo.Object{}, control.ErrProtocol
		}
		if progress != nil {
			if _, err := io.WriteString(progress, frame.Progress+"\n"); err != nil {
				return gitrepo.Object{}, err
			}
		}
	}
}
