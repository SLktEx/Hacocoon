package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const MethodSnapshot = "snapshot.manage"

var publicSnapshotID = regexp.MustCompile(`^snap-[a-f0-9]{32}$`)

type SnapshotRequest struct {
	Operation   string `json:"operation"`
	Environment string `json:"environment,omitempty"`
	ID          string `json:"id,omitempty"`
}
type SnapshotSummary struct {
	ID          string `json:"id"`
	Environment string `json:"environment"`
	State       string `json:"state"`
	Workspaces  int    `json:"workspaces"`
	OCI         bool   `json:"oci"`
}
type SnapshotResponse struct {
	Snapshots []SnapshotSummary `json:"snapshots"`
	Error     *responseStatus   `json:"error,omitempty"`
}
type snapshotService interface {
	CaptureSnapshot(context.Context, string) (core.Snapshot, error)
	ListSnapshots(context.Context, string) ([]core.Snapshot, error)
	DeleteSnapshot(context.Context, string) error
}

func RegisterSnapshots(server *control.Server, service snapshotService) error {
	return server.Register(MethodSnapshot, func(ctx context.Context, payload json.RawMessage) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
		defer cancel()
		var req SnapshotRequest
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil {
			return nil, translateError(core.ErrInvalidArgument)
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return nil, translateError(core.ErrInvalidArgument)
		}
		response := SnapshotResponse{Snapshots: []SnapshotSummary{}}
		var err error
		switch req.Operation {
		case "create":
			if req.ID != "" || strings.TrimSpace(req.Environment) == "" {
				return nil, translateError(core.ErrInvalidArgument)
			}
			var saved core.Snapshot
			saved, err = service.CaptureSnapshot(ctx, req.Environment)
			if saved.ID != "" {
				response.Snapshots = append(response.Snapshots, summarizeSnapshot(saved))
			}
		case "list":
			if req.ID != "" {
				return nil, translateError(core.ErrInvalidArgument)
			}
			var saved []core.Snapshot
			saved, err = service.ListSnapshots(ctx, req.Environment)
			for _, item := range saved {
				response.Snapshots = append(response.Snapshots, summarizeSnapshot(item))
			}
		case "delete":
			if req.Environment != "" || !publicSnapshotID.MatchString(req.ID) {
				return nil, translateError(core.ErrInvalidArgument)
			}
			err = service.DeleteSnapshot(ctx, req.ID)
		default:
			return nil, translateError(core.ErrInvalidArgument)
		}
		// Keep a saved/partial ID even when capture or subsequent restart failed.
		response.Error = statusFromError(err)
		return response, nil
	})
}
func summarizeSnapshot(saved core.Snapshot) SnapshotSummary {
	summary := SnapshotSummary{ID: saved.ID, Environment: saved.Source.Environment.Name, State: saved.State}
	for _, component := range saved.Components {
		if strings.HasPrefix(component.Role, "workspace:") {
			summary.Workspaces++
		}
		if component.Role == "oci" {
			summary.OCI = true
		}
	}
	return summary
}
func (c *Client) Snapshot(ctx context.Context, req SnapshotRequest) (SnapshotResponse, error) {
	var response SnapshotResponse
	if err := c.wire.Call(ctx, MethodSnapshot, req, &response); err != nil {
		return response, err
	}
	return response, responseError(response.Error)
}
