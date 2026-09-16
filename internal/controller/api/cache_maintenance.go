package controlapi

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
)

const MethodCacheHistory = "cache.history"
const MethodCacheClear = "cache.clear"
const MethodCacheRecover = "cache.recover"

type CacheMaintenanceRequest struct {
	Environment string `json:"environment"`
	Area        string `json:"area"`
	Revision    string `json:"revision,omitempty"`
}

type CacheMaintenanceResponse struct {
	Recovery cache.RecoveryResult `json:"recovery"`
	History  cache.History        `json:"history"`
	Result   cache.ClearResult    `json:"result"`
	Failure  string               `json:"failure,omitempty"`
}

func registerCacheMaintenance(server *control.Server, workflow *cache.Workflow) error {
	for _, method := range []string{MethodCacheHistory, MethodCacheClear, MethodCacheRecover} {
		if err := server.Register(method, func(ctx context.Context, payload json.RawMessage) (any, error) {
			var request CacheMaintenanceRequest
			if !decodeCacheRequest(payload, &request) || request.Environment == "" || len(request.Environment) > 128 || request.Area == "" || len(request.Area) > 64 || method != MethodCacheClear && request.Revision != "" || method == MethodCacheClear && len(request.Revision) != 64 {
				return nil, control.ErrInvalidArgument
			}
			ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			var response CacheMaintenanceResponse
			var err error
			switch method {
			case MethodCacheHistory:
				response.History, err = workflow.History(ctx, request.Environment, request.Area)
			case MethodCacheRecover:
				response.Recovery, err = workflow.Recover(ctx, request.Environment, request.Area)
			case MethodCacheClear:
				response.Result, err = workflow.Clear(ctx, request.Environment, request.Area, request.Revision)
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
	if err := registerCacheCatalog(server, workflow); err != nil {
		return err
	}
	return registerCacheEmpty(server, workflow)
}

func (c *Client) CacheHistory(ctx context.Context, name, area string) (CacheMaintenanceResponse, error) {
	var response CacheMaintenanceResponse
	err := c.wire.Call(ctx, MethodCacheHistory, CacheMaintenanceRequest{Environment: name, Area: area}, &response)
	return response, err
}
func (c *Client) ClearCache(ctx context.Context, name, area, revision string) (CacheMaintenanceResponse, error) {
	var response CacheMaintenanceResponse
	err := c.wire.Call(ctx, MethodCacheClear, CacheMaintenanceRequest{Environment: name, Area: area, Revision: revision}, &response)
	return response, err
}

func (c *Client) RecoverCache(ctx context.Context, name, area string) (CacheMaintenanceResponse, error) {
	var response CacheMaintenanceResponse
	err := c.wire.Call(ctx, MethodCacheRecover, CacheMaintenanceRequest{Environment: name, Area: area}, &response)
	return response, err
}
