package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmentcopy"
)

const MethodEnvironmentCopy = "environment.copy"

type EnvironmentCopyRequest struct {
	Source string `json:"source"`
	Target string `json:"target,omitempty"`
}
type EnvironmentCopyResponse struct {
	Result environmentcopy.Result `json:"result"`
	Error  *responseStatus        `json:"error,omitempty"`
}
type environmentCopier interface {
	CopyEnvironment(context.Context, string, string) (environmentcopy.Result, error)
}

func RegisterEnvironmentCopy(server *control.Server, service environmentCopier) error {
	return server.Register(MethodEnvironmentCopy, func(ctx context.Context, payload json.RawMessage) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
		var req EnvironmentCopyRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF || req.Source == "" {
			return nil, translateError(core.ErrInvalidArgument)
		}
		result, err := service.CopyEnvironment(ctx, req.Source, req.Target)
		return EnvironmentCopyResponse{Result: result, Error: statusFromError(err)}, nil
	})
}
func (c *Client) CopyEnvironment(ctx context.Context, req EnvironmentCopyRequest) (EnvironmentCopyResponse, error) {
	var response EnvironmentCopyResponse
	if err := c.wire.Call(ctx, MethodEnvironmentCopy, req, &response); err != nil {
		return response, err
	}
	return response, responseError(response.Error)
}
