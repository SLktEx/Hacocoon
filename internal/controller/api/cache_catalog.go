package controlapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
)

const MethodCacheCatalog = "cache.catalog"

type CacheCatalogResponse struct {
	Catalog cache.CatalogHistory `json:"catalog"`
	Failure string               `json:"failure,omitempty"`
}
type cacheCatalogRequest struct {
	Operation string `json:"operation"`
	Revision  string `json:"revision,omitempty"`
}

func registerCacheCatalog(server *control.Server, workflow *cache.Workflow) error {
	return server.Register(MethodCacheCatalog, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req cacheCatalogRequest
		if !decodeCacheRequest(payload, &req) || req.Operation != "history" && req.Operation != "clear" && req.Operation != "recover" || req.Operation == "history" && req.Revision != "" || req.Operation != "history" && len(req.Revision) != 64 {
			return nil, control.ErrInvalidArgument
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		var response CacheCatalogResponse
		var err error
		if req.Operation == "history" {
			response.Catalog, err = workflow.CatalogHistory(ctx)
		} else {
			response.Catalog, err = workflow.MaintainCatalog(ctx, req.Revision, req.Operation == "clear")
		}
		response.Failure = cache.WorkflowError(err)
		if err != nil {
			logging.FromContext(ctx).ErrorContext(ctx, "Cache operation failed", "component", "cache", "operation", MethodCacheCatalog, "action", req.Operation, "failure_code", response.Failure)
		}
		return response, nil
	})
}
func (c *Client) MaintainCacheCatalog(ctx context.Context, operation, revision string) (CacheCatalogResponse, error) {
	var response CacheCatalogResponse
	err := c.wire.Call(ctx, MethodCacheCatalog, cacheCatalogRequest{Operation: operation, Revision: revision}, &response)
	return response, err
}
