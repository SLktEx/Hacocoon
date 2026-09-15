package controlapi

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/modules/standard/cache"
	"time"
)

const MethodCacheEmptyPreview = "cache.empty.preview"
const MethodCacheEmpty = "cache.empty"

type cacheEmptyRequest struct {
	Scope    cache.EmptyScope `json:"scope"`
	Revision string           `json:"revision,omitempty"`
}
type CacheEmptyResponse struct {
	Preview cache.EmptyPreview `json:"preview"`
	Failure string             `json:"failure,omitempty"`
}

func registerCacheEmpty(server *control.Server, workflow *cache.Workflow) error {
	for _, method := range []string{MethodCacheEmptyPreview, MethodCacheEmpty} {
		if err := server.Register(method, func(ctx context.Context, payload json.RawMessage) (any, error) {
			var req cacheEmptyRequest
			if !decodeCacheRequest(payload, &req) || !req.Scope.Valid() || method == MethodCacheEmptyPreview && req.Revision != "" || method == MethodCacheEmpty && len(req.Revision) != 64 {
				return nil, control.ErrInvalidArgument
			}
			ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			var response CacheEmptyResponse
			var err error
			if method == MethodCacheEmptyPreview {
				response.Preview, err = workflow.PreviewEmpty(ctx, req.Scope)
			} else {
				response.Preview, err = workflow.Empty(ctx, req.Scope, req.Revision)
			}
			response.Failure = cache.WorkflowError(err)
			if err != nil {
				logging.FromContext(ctx).ErrorContext(ctx, "Cache operation failed", "component", "cache", "operation", method, "failure_code", response.Failure)
			}
			return response, nil
		}); err != nil {
			return err
		}
	}
	return nil
}
func (c *Client) PreviewEmptyCache(ctx context.Context, scope cache.EmptyScope) (CacheEmptyResponse, error) {
	var response CacheEmptyResponse
	err := c.wire.Call(ctx, MethodCacheEmptyPreview, cacheEmptyRequest{Scope: scope}, &response)
	return response, err
}
func (c *Client) EmptyCache(ctx context.Context, scope cache.EmptyScope, revision string) (CacheEmptyResponse, error) {
	var response CacheEmptyResponse
	err := c.wire.Call(ctx, MethodCacheEmpty, cacheEmptyRequest{Scope: scope, Revision: revision}, &response)
	return response, err
}
