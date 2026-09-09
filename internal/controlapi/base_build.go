package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"time"
)

const MethodBaseBuild = "base.build"

type BaseBuildResponse struct {
	Result basebuild.Result `json:"result"`
	Error  *responseStatus  `json:"error,omitempty"`
}
type baseBuilder interface {
	Build(context.Context, basebuild.Definition) (basebuild.Result, error)
}

func RegisterBaseBuild(server *control.Server, service baseBuilder) error {
	return server.Register(MethodBaseBuild, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var d basebuild.Definition
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&d) != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF || d.Validate() != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		result, err := service.Build(ctx, d)
		return BaseBuildResponse{Result: result, Error: statusFromError(err)}, nil
	})
}
func (c *Client) BuildBase(ctx context.Context, d basebuild.Definition) (BaseBuildResponse, error) {
	var response BaseBuildResponse
	if err := c.wire.Call(ctx, MethodBaseBuild, d, &response); err != nil {
		return response, err
	}
	return response, responseError(response.Error)
}
