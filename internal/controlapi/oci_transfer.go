package controlapi

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

const MethodOCIDistribute = "plugin.oci.distribute"

func RegisterOCITransfer(server *control.Server, service *oci.TransferService) error {
	return server.Register(MethodOCIDistribute, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req oci.TransferRequest
		if json.Unmarshal(payload, &req) != nil {
			return nil, control.ErrInvalidArgument
		}
		result, err := service.Distribute(ctx, req)
		return result, translateError(err)
	})
}
func (c *Client) DistributeImage(ctx context.Context, req oci.TransferRequest) (oci.TransferReport, error) {
	var result oci.TransferReport
	err := c.wire.Call(ctx, MethodOCIDistribute, req, &result)
	return result, err
}
