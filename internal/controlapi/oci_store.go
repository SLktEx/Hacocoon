package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

const MethodOCIStore = "plugin.oci.store"

type OCIStoreRequest struct {
	Owner     string `json:"owner,omitempty"`
	Operation string `json:"operation"`
	ID        string `json:"id,omitempty"`
	From      string `json:"from,omitempty"`
}
type OCIStoreResponse struct {
	Uses      []oci.StoreUse            `json:"uses,omitempty"`
	Resources []core.PersistentResource `json:"resources"`
}

func RegisterOCIStores(server *control.Server, service *persistentresource.Service) error {
	return server.Register(MethodOCIStore, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req OCIStoreRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF {
			return nil, translateError(core.ErrInvalidArgument)
		}
		if (req.Owner != "" && req.Operation != "delete") || (req.Operation == "delete" && !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: req.ID, Owner: req.Owner})) {
			return nil, translateError(core.ErrInvalidArgument)
		}
		ctx, cancel := context.WithTimeout(ctx, 12*time.Minute)
		defer cancel()
		if req.From != "" && (req.Operation != "create" || !strings.HasPrefix(req.From, "oci:") || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: req.From, Owner: strings.Repeat("0", 32)})) {
			return nil, translateError(core.ErrInvalidArgument)
		}
		if req.Operation == "list" {
			if req.ID != "" {
				return nil, translateError(core.ErrInvalidArgument)
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
			catalog, ok := service.Store.(oci.StoreReferenceCatalog)
			if !ok {
				return nil, translateError(core.ErrUnsupported)
			}
			result.Uses, err = oci.StoreUses(ctx, catalog, all)
			if err != nil {
				return nil, translateError(err)
			}
			return result, nil
		}
		// Plugin requests cannot select arbitrary resource kinds or provider names.
		if !strings.HasPrefix(req.ID, "oci:") || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: req.ID, Owner: strings.Repeat("0", 32)}) {
			return nil, translateError(core.ErrInvalidArgument)
		}
		var resource core.PersistentResource
		var err error
		switch req.Operation {
		case "create":
			if req.From != "" {
				resource, err = service.Copy(ctx, req.ID, oci.StoreKind, req.From)
			} else {
				resource, err = service.Create(ctx, req.ID, oci.StoreKind)
			}
		case "inspect", "delete":
			resource, err = service.Store.GetPersistentResource(ctx, req.ID)
			if err == nil && resource.Kind != oci.StoreKind {
				err = core.ErrIncompatibleState
			}
			if err == nil && req.Operation == "delete" {
				err = service.DeleteReviewed(ctx, core.PersistentResourceRef{ID: req.ID, Owner: req.Owner})
				resource.State = "deleted"
			}
		default:
			return nil, translateError(core.ErrInvalidArgument)
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
