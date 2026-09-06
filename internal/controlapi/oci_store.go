package controlapi

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

const MethodOCIStore = "plugin.oci.store"

type OCIStoreRequest struct {
	Operation string `json:"operation"`
	ID        string `json:"id,omitempty"`
}
type OCIStoreResponse struct {
	Resources []core.PersistentResource `json:"resources"`
}

func RegisterOCIStores(server *control.Server, service *persistentresource.Service) error {
	return server.Register(MethodOCIStore, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req OCIStoreRequest
		if json.Unmarshal(payload, &req) != nil {
			return nil, control.ErrInvalidArgument
		}
		if req.Operation == "list" {
			if req.ID != "" {
				return nil, control.ErrInvalidArgument
			}
			all, err := service.Store.ListPersistentResources(ctx)
			if err != nil {
				return nil, translateError(err)
			}
			result := OCIStoreResponse{Resources: []core.PersistentResource{}}
			for _, r := range all {
				if r.Kind == oci.StoreKind {
					result.Resources = append(result.Resources, r)
				}
			}
			return result, nil
		}
		// Plugin requests cannot select arbitrary resource kinds or provider names.
		if !strings.HasPrefix(req.ID, "oci:") || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: req.ID, Owner: strings.Repeat("0", 32)}) {
			return nil, control.ErrInvalidArgument
		}
		var resource core.PersistentResource
		var err error
		switch req.Operation {
		case "create":
			resource, err = service.Create(ctx, req.ID, oci.StoreKind)
		case "inspect", "delete":
			resource, err = service.Store.GetPersistentResource(ctx, req.ID)
			if err == nil && resource.Kind != oci.StoreKind {
				err = core.ErrIncompatibleState
			}
			if err == nil && req.Operation == "delete" {
				err = service.Delete(ctx, req.ID)
				resource.State = "deleted"
			}
		default:
			return nil, control.ErrInvalidArgument
		}
		if err != nil {
			return nil, translateError(err)
		}
		return OCIStoreResponse{Resources: []core.PersistentResource{resource}}, nil
	})
}

func (c *Client) OCIStore(ctx context.Context, req OCIStoreRequest) (OCIStoreResponse, error) {
	var response OCIStoreResponse
	err := c.wire.Call(ctx, MethodOCIStore, req, &response)
	return response, err
}
