package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
)

const MethodImageCommand = "image.command"

type ImageCommandRequest struct {
	Operation string        `json:"operation"`
	Source    string        `json:"source"`
	Target    core.BaseName `json:"target,omitempty"`
}
type imageCommands interface {
	TagImage(context.Context, core.BaseName, core.BaseName) (core.BaseInfo, error)
	RemoveImageTag(context.Context, core.BaseName) error
}
type imageCommitter interface {
	Commit(context.Context, string, core.BaseName) (core.BaseInfo, error)
}

func RegisterImageCommands(server *control.Server, images imageCommands, environments imageCommitter) error {
	return server.Register(MethodImageCommand, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req ImageCommandRequest
		d := json.NewDecoder(bytes.NewReader(payload))
		d.DisallowUnknownFields()
		if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF || req.Source == "" {
			return nil, translateError(core.ErrInvalidArgument)
		}
		var result core.BaseInfo
		var err error
		switch req.Operation {
		case "tag":
			result, err = images.TagImage(ctx, core.BaseName(req.Source), req.Target)
		case "commit":
			result, err = environments.Commit(ctx, req.Source, req.Target)
		case "untag":
			if req.Target != "" {
				return nil, translateError(core.ErrInvalidArgument)
			}
			err = images.RemoveImageTag(ctx, core.BaseName(req.Source))
		default:
			err = core.ErrInvalidArgument
		}
		return result, translateError(err)
	})
}
func (c *Client) ImageCommand(ctx context.Context, req ImageCommandRequest) (core.BaseInfo, error) {
	var result core.BaseInfo
	err := c.wire.Call(ctx, MethodImageCommand, req, &result)
	return result, err
}
