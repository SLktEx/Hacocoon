package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

const MethodReclamationTarget = "storage.reclamation-target"

type reclamationTargetService interface {
	ReclamationTarget(context.Context) (reclamation.WSLTarget, error)
}
type ReclamationTargetResponse struct {
	ProtocolVersion int `json:"protocol_version"`
	reclamation.WSLTarget
}

// Management-only observation for a CLI with no required GUID/path arguments.
// The Windows helper still requires its independently saved native enrollment.
func RegisterReclamationTarget(server *control.Server, service reclamationTargetService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	return server.Register(MethodReclamationTarget, func(ctx context.Context, payload json.RawMessage) (any, error) {
		if !bytes.Equal(bytes.TrimSpace(payload), []byte("{}")) {
			return nil, control.NewStatusError("invalid_argument", "target discovery accepts no selection")
		}
		ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		target, err := service.ReclamationTarget(ctx)
		if err != nil || ctx.Err() != nil {
			return nil, control.NewStatusError("unavailable", "managed Windows installation unavailable")
		}
		if target.Validate() != nil {
			return nil, control.NewStatusError("unavailable", "managed Windows installation identity invalid")
		}
		return ReclamationTargetResponse{ProtocolVersion: control.ProtocolVersion, WSLTarget: target}, nil
	})
}

func (c *Client) ReclamationTarget(ctx context.Context) (reclamation.WSLTarget, error) {
	var response ReclamationTargetResponse
	if err := c.wire.Call(ctx, MethodReclamationTarget, struct{}{}, &response); err != nil {
		return reclamation.WSLTarget{}, err
	}
	if response.ProtocolVersion != control.ProtocolVersion || response.WSLTarget.Validate() != nil {
		return reclamation.WSLTarget{}, control.ErrProtocol
	}
	return response.WSLTarget, nil
}
