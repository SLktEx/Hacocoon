package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"io"
	"time"
)

const MethodOCIImage = "plugin.oci.image"

type OCIImageRequest struct {
	Host        bool            `json:"host,omitempty"`
	Operation   string          `json:"operation"`
	Environment string          `json:"environment,omitempty"`
	Runtime     string          `json:"runtime,omitempty"`
	Target      oci.ImageTarget `json:"target,omitempty"`
	ID          string          `json:"id,omitempty"`
}
type OCIImageService interface {
	ListHost(context.Context, string) (oci.ManagedImageList, error)
	List(context.Context, string, string) (oci.ManagedImageList, error)
	Delete(context.Context, oci.ImageTarget, string) error
}

func RegisterOCIImages(server *control.Server, service OCIImageService) error {
	return server.Register(MethodOCIImage, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req OCIImageRequest
		d := json.NewDecoder(bytes.NewReader(payload))
		d.DisallowUnknownFields()
		if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF {
			return nil, translateError(core.ErrInvalidArgument)
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		switch req.Operation {
		case "list":
			if (req.Host && req.Environment != "") || (!req.Host && req.Environment == "") || req.ID != "" || req.Target != (oci.ImageTarget{}) || (req.Runtime != "docker" && req.Runtime != "nerdctl") {
				return nil, translateError(core.ErrInvalidArgument)
			}
			var result oci.ManagedImageList
			var err error
			if req.Host {
				result, err = service.ListHost(ctx, req.Runtime)
			} else {
				result, err = service.List(ctx, req.Environment, req.Runtime)
			}
			return result, translateError(err)
		case "delete":
			if req.Host || req.Environment != "" || req.Runtime != "" || !oci.ValidImageSelection(req.Target, req.ID) {
				return nil, translateError(core.ErrInvalidArgument)
			}
			return struct{}{}, translateError(service.Delete(ctx, req.Target, req.ID))
		default:
			return nil, translateError(core.ErrInvalidArgument)
		}
	})
}
func (c *Client) OCIImage(ctx context.Context, req OCIImageRequest) (oci.ManagedImageList, error) {
	var result oci.ManagedImageList
	err := c.wire.Call(ctx, MethodOCIImage, req, &result)
	return result, err
}
