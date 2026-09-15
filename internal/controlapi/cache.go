package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/modules/standard/cache"
	"io"
	"time"
)

const MethodCacheSettings = "cache.settings"
const MethodCacheConfigure = "cache.configure"
const MethodCacheStatus = "cache.status"
const MethodCacheCollect = "cache.collect"

type CacheRequest struct {
	Environment string `json:"environment"`
	Area        string `json:"area,omitempty"`
}
type CacheResponse struct {
	Areas   []cache.AreaStatus `json:"areas"`
	Failure string             `json:"failure,omitempty"`
}

// Cache settings and collection exist only on the trusted management transport.
// The ordinary guest Git/network/notification interfaces do not register them.
func RegisterCache(server *control.Server, workflow *cache.Workflow) error {
	if server == nil || workflow == nil || workflow.Catalog == nil || workflow.Collector == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodCacheSettings, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if len(payload) != 0 && string(payload) != "null" && string(payload) != "{}" {
			return nil, control.ErrInvalidArgument
		}
		result, err := workflow.Settings.Read(ctx)
		return result, configurationError(err)
	}); err != nil {
		return err
	}
	if err := server.Register(MethodCacheConfigure, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request struct {
			Revision      string          `json:"revision"`
			Configuration json.RawMessage `json:"configuration"`
		}
		if !decodeCacheRequest(payload, &request) {
			return nil, control.ErrInvalidArgument
		}
		configuration, err := cache.DecodeConfiguration(request.Configuration)
		if err != nil {
			return nil, control.ErrInvalidArgument
		}
		result, err := workflow.Settings.Replace(ctx, cache.SettingsSnapshot{Revision: request.Revision, Configuration: configuration})
		return result, configurationError(err)
	}); err != nil {
		return err
	}
	for _, method := range []string{MethodCacheStatus, MethodCacheCollect} {
		if err := server.Register(method, func(ctx context.Context, payload json.RawMessage) (any, error) {
			var request CacheRequest
			if !decodeCacheRequest(payload, &request) || request.Environment == "" || len(request.Environment) > 128 || len(request.Area) > 64 || method == MethodCacheStatus && request.Area != "" {
				return nil, control.ErrInvalidArgument
			}
			ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			defer cancel()
			var areas []cache.AreaStatus
			var err error
			if method == MethodCacheStatus {
				areas, err = workflow.Status(ctx, request.Environment)
			} else {
				areas, err = workflow.Collect(ctx, request.Environment, request.Area)
			}
			if err != nil {
				logging.FromContext(ctx).ErrorContext(ctx, "Cache operation failed", "component", "cache", "operation", method, "failure_code", cache.WorkflowError(err))
			}
			return CacheResponse{Areas: areas, Failure: cache.WorkflowError(err)}, nil
		}); err != nil {
			return err
		}
	}
	return registerCacheMaintenance(server, workflow)
}
func decodeCacheRequest(data []byte, target any) bool {
	if len(data) > cache.MaxConfigurationBytes+1024 {
		return false
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	return d.Decode(target) == nil && d.Decode(new(any)) == io.EOF
}
func (c *Client) CacheSettings(ctx context.Context) (cache.SettingsSnapshot, error) {
	var r cache.SettingsSnapshot
	err := c.wire.Call(ctx, MethodCacheSettings, nil, &r)
	return r, err
}
func (c *Client) ConfigureCache(ctx context.Context, edit cache.SettingsSnapshot) (cache.SettingsSnapshot, error) {
	var r cache.SettingsSnapshot
	err := c.wire.Call(ctx, MethodCacheConfigure, edit, &r)
	return r, err
}
func (c *Client) CacheStatus(ctx context.Context, name string) (CacheResponse, error) {
	var r CacheResponse
	err := c.wire.Call(ctx, MethodCacheStatus, CacheRequest{Environment: name}, &r)
	return r, err
}
func (c *Client) CollectCache(ctx context.Context, name, area string) (CacheResponse, error) {
	var r CacheResponse
	err := c.wire.Call(ctx, MethodCacheCollect, CacheRequest{Environment: name, Area: area}, &r)
	return r, err
}
