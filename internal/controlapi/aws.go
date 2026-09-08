package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"io"
)

const MethodAWSList = "aws.s3.list"

type awsListService interface {
	List(context.Context, awsplugin.ListSpec) (core.CapabilityResult, error)
}
type awsResponse struct {
	Result core.CapabilityResult `json:"result"`
	Error  *responseStatus       `json:"error,omitempty"`
}

func RegisterAWS(server *control.Server, service awsListService) error {
	if service == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodAWSList, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var spec awsplugin.ListSpec
		d := json.NewDecoder(bytes.NewReader(payload))
		d.DisallowUnknownFields()
		if d.Decode(&spec) != nil || d.Decode(new(any)) != io.EOF {
			return nil, control.ErrInvalidArgument
		}
		result, err := service.List(ctx, spec)
		return awsResponse{Result: result, Error: statusFromError(err)}, nil
	}); err != nil {
		return err
	}
	if stream, ok := service.(awsDownloadService); ok {
		return registerAWSDownload(server, stream)
	}
	return nil
}
func (c *Client) ListS3(ctx context.Context, spec awsplugin.ListSpec) (core.CapabilityResult, error) {
	var response awsResponse
	if err := c.wire.Call(ctx, MethodAWSList, spec, &response); err != nil {
		return response.Result, err
	}
	return response.Result, responseError(response.Error)
}
