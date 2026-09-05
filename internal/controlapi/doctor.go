package controlapi

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/buildinfo"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/diagnostics"
)

const MethodDoctor = "system.doctor"

type DoctorResponse struct {
	ProtocolVersion int            `json:"protocol_version"`
	Controller      buildinfo.Info `json:"controller"`
	diagnostics.Report
}

type diagnosticService interface {
	DiagnoseHost(context.Context) (diagnostics.Report, error)
}

func RegisterDoctor(server *control.Server, service diagnosticService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	return server.Register(MethodDoctor, func(ctx context.Context, payload json.RawMessage) (any, error) {
		// No caller-selected paths, commands, targets, or repair options.
		switch strings.TrimSpace(string(payload)) {
		case "", "null", "{}":
		default:
			return nil, control.NewStatusError("invalid_argument", "doctor accepts no parameters")
		}
		// This bound also applies to direct RPC callers, not just the CLI.
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		report, err := service.DiagnoseHost(ctx)
		if err != nil || report.Validate() != nil {
			return nil, control.NewStatusError("diagnostics_failed", "Host diagnostics could not be completed")
		}
		return DoctorResponse{ProtocolVersion: control.ProtocolVersion, Controller: buildinfo.Current(), Report: report}, nil
	})
}

func (c *Client) Doctor(ctx context.Context) (DoctorResponse, error) {
	var response DoctorResponse
	if err := c.wire.Call(ctx, MethodDoctor, nil, &response); err != nil {
		return response, err
	}
	if response.ProtocolVersion != control.ProtocolVersion || response.Report.Validate() != nil || !validDoctorBuild(response.Controller) {
		return DoctorResponse{}, control.ErrProtocol
	}
	return response, nil
}

func validDoctorBuild(info buildinfo.Info) bool {
	for _, value := range []string{info.Checkpoint, info.Version, info.Commit, info.BuildDate} {
		if len(value) == 0 || len(value) > 128 {
			return false
		}
		for _, c := range value {
			if c < 32 || c > 126 {
				return false
			}
		}
	}
	return true
}
