package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
	"io"
)

func strictDecode(payload json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return core.ErrInvalidArgument
	}
	return nil
}

const MethodCreate = "environment.create_source"
const MethodOpen = "environment.open"
const MethodImageDefault = "image.default"

type OpenRequest struct {
	Environment string            `json:"environment,omitempty"`
	New         *creation.Request `json:"new,omitempty"`
}
type creationService interface {
	Create(context.Context, creation.Request) (core.Environment, error)
	OpenTarget(context.Context, string, *creation.Request) (core.Environment, error)
	DefaultImage(context.Context, core.BaseName) (core.BaseName, error)
}

func RegisterCreation(server *control.Server, service creationService) error {
	if server == nil || service == nil {
		return core.ErrInvalidArgument
	}
	if err := server.Register(MethodCreate, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request creation.Request
		if err := strictDecode(payload, &request); err != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		result, err := service.Create(ctx, request)
		return result, translateError(err)
	}); err != nil {
		return err
	}
	if err := server.Register(MethodOpen, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request OpenRequest
		if err := strictDecode(payload, &request); err != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		result, err := service.OpenTarget(ctx, request.Environment, request.New)
		return result, translateError(err)
	}); err != nil {
		return err
	}
	return server.Register(MethodImageDefault, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request struct {
			Image core.BaseName `json:"image,omitempty"`
		}
		if err := strictDecode(payload, &request); err != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		result, err := service.DefaultImage(ctx, request.Image)
		return result, translateError(err)
	})
}
func (c *Client) Create(ctx context.Context, request creation.Request) (result core.Environment, err error) {
	err = c.wire.Call(ctx, MethodCreate, request, &result)
	return
}
func (c *Client) OpenTarget(ctx context.Context, request OpenRequest) (result core.Environment, err error) {
	err = c.wire.Call(ctx, MethodOpen, request, &result)
	return
}
func (c *Client) DefaultImage(ctx context.Context, image core.BaseName) (result core.BaseName, err error) {
	err = c.wire.Call(ctx, MethodImageDefault, struct {
		Image core.BaseName `json:"image,omitempty"`
	}{image}, &result)
	return
}
