package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/basemanage"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"time"
)

const MethodBaseManage = "base.manage"

type BaseManageRequest struct {
	Operation string              `json:"operation"`
	Image     basemanage.Identity `json:"image"`
}
type baseManageService interface {
	List(context.Context) ([]basemanage.Image, error)
	Delete(context.Context, basemanage.Identity) error
}

func RegisterBaseManage(server *control.Server, service baseManageService) error {
	return server.Register(MethodBaseManage, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req BaseManageRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
			return nil, translateError(core.ErrInvalidArgument)
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		switch req.Operation {
		case "list":
			if req.Image != (basemanage.Identity{}) {
				return nil, translateError(core.ErrInvalidArgument)
			}
			v, err := service.List(ctx)
			return v, translateError(err)
		case "delete":
			if req.Image.Name == "" || req.Image.Fingerprint == "" || req.Image.BuildInstance == "" {
				return nil, translateError(core.ErrInvalidArgument)
			}
			return nil, translateError(service.Delete(ctx, req.Image))
		default:
			return nil, translateError(core.ErrInvalidArgument)
		}
	})
}
func (c *Client) ListBaseImages(ctx context.Context) ([]basemanage.Image, error) {
	var result []basemanage.Image
	err := c.wire.Call(ctx, MethodBaseManage, BaseManageRequest{Operation: "list"}, &result)
	return result, err
}
func (c *Client) DeleteBaseImage(ctx context.Context, id basemanage.Identity) error {
	return c.wire.Call(ctx, MethodBaseManage, BaseManageRequest{Operation: "delete", Image: id}, nil)
}
