package controlapi

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
)

const MethodVolume = "volume.manage"

type VolumeRequest struct {
	Operation string `json:"operation"`
	Name      string `json:"name,omitempty"`
	Container string `json:"container,omitempty"`
}
type volumeService interface {
	Volumes(context.Context) ([]creation.Volume, error)
	CreateVolume(context.Context, string, string) (creation.Volume, error)
	InspectVolume(context.Context, string) (creation.Volume, error)
	DeleteVolume(context.Context, string) error
}

func RegisterVolumes(server *control.Server, service volumeService) error {
	return server.Register(MethodVolume, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var r VolumeRequest
		if strictDecode(payload, &r) != nil || (r.Operation != "create" && r.Container != "") {
			return nil, translateError(core.ErrInvalidArgument)
		}
		var result []creation.Volume
		var item creation.Volume
		var err error
		switch r.Operation {
		case "ls":
			if r.Name != "" {
				return nil, translateError(core.ErrInvalidArgument)
			}
			result, err = service.Volumes(ctx)
		case "create":
			item, err = service.CreateVolume(ctx, r.Name, r.Container)
			result = []creation.Volume{item}
		case "inspect":
			item, err = service.InspectVolume(ctx, r.Name)
			result = []creation.Volume{item}
		case "rm":
			err = service.DeleteVolume(ctx, r.Name)
		default:
			err = core.ErrInvalidArgument
		}
		return result, translateError(err)
	})
}
func (c *Client) Volume(ctx context.Context, request VolumeRequest) (result []creation.Volume, err error) {
	err = c.wire.Call(ctx, MethodVolume, request, &result)
	return
}
