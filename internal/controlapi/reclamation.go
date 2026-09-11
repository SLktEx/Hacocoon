package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

const MethodReclaimLinux = "storage.reclaim-linux"

type reclamationService interface {
	ReclaimLinux(context.Context, reclamation.WSLTarget) reclamation.LinuxReport
}
type ReclaimLinuxResponse struct {
	ProtocolVersion int `json:"protocol_version"`
	reclamation.LinuxReport
}

// This registration belongs only to the management endpoint, never a guest
// Git/notification endpoint. An RPC result is not permission for Windows action.
func RegisterReclamation(server *control.Server, service reclamationService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	active := make(chan struct{}, 1)
	return server.Register(MethodReclaimLinux, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var target reclamation.WSLTarget
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&target) != nil || target.Validate() != nil {
			return nil, control.NewStatusError("invalid_argument", "exact WSL identity required")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return nil, control.NewStatusError("invalid_argument", "invalid reclamation request")
		}
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			return nil, control.NewStatusError("busy", "storage reclamation is already running")
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		report := service.ReclaimLinux(ctx, target)
		if ctx.Err() != nil && report.Failure == "" {
			report.Failure = "canceled"
		}
		if report.Validate() != nil {
			logging.Root().ErrorContext(ctx, "Invalid reclamation result", "component", "control", "operation", "reclaim_storage")
			return nil, control.NewStatusError("reclamation_failed", "storage reclamation result is unavailable")
		}
		if !report.Complete() {
			logging.Root().ErrorContext(ctx, "Storage reclamation failed", "component", "control", "operation", "reclaim_storage", "reason", report.Failure)
		}
		// Keep partial observations in an explicit failed report, rather than losing
		// an attempted operation behind a transport error or reporting overall success.
		return ReclaimLinuxResponse{ProtocolVersion: control.ProtocolVersion, LinuxReport: report}, nil
	})
}
func (c *Client) ReclaimLinux(ctx context.Context, target reclamation.WSLTarget) (ReclaimLinuxResponse, error) {
	if err := target.Validate(); err != nil {
		return ReclaimLinuxResponse{}, err
	}
	var response ReclaimLinuxResponse
	if err := c.wire.Call(ctx, MethodReclaimLinux, target, &response); err != nil {
		return response, err
	}
	if response.ProtocolVersion != control.ProtocolVersion || response.Validate() != nil {
		return ReclaimLinuxResponse{}, control.ErrProtocol
	}
	return response, nil
}
