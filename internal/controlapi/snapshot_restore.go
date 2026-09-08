package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/snapshotrestore"
)

const MethodSnapshotRestore = "snapshot.restore"

type SnapshotRestoreRequest struct {
	ID          string `json:"id"`
	Environment string `json:"environment,omitempty"`
}
type SnapshotRestoreResponse struct {
	Result snapshotrestore.Result `json:"result"`
	Error  *responseStatus        `json:"error,omitempty"`
}
type snapshotRestorer interface {
	RestoreSnapshot(context.Context, string, string) (snapshotrestore.Result, error)
}

func RegisterSnapshotRestore(server *control.Server, service snapshotRestorer) error {
	return server.Register(MethodSnapshotRestore, func(ctx context.Context, payload json.RawMessage) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
		var req SnapshotRestoreRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF || !publicSnapshotID.MatchString(req.ID) {
			return nil, translateError(core.ErrInvalidArgument)
		}
		result, err := service.RestoreSnapshot(ctx, req.ID, req.Environment)
		return SnapshotRestoreResponse{Result: result, Error: statusFromError(err)}, nil
	})
}
func (c *Client) RestoreSnapshot(ctx context.Context, req SnapshotRestoreRequest) (SnapshotRestoreResponse, error) {
	var response SnapshotRestoreResponse
	if err := c.wire.Call(ctx, MethodSnapshotRestore, req, &response); err != nil {
		return response, err
	}
	return response, responseError(response.Error)
}
