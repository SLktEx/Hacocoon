package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"io"
	"time"
)

const MethodRepositoryManage = "repository.manage"

type RepositoryManageRequest struct {
	Operation string `json:"operation"`
	ID        string `json:"id,omitempty"`
	Owner     string `json:"owner,omitempty"`
}
type RepositoryManageResponse struct {
	Sources []gitrepo.SourceUse `json:"sources"`
}

func repositoryManageHandler(s *gitrepo.RepositoryService) control.Handler {
	return func(ctx context.Context, payload json.RawMessage) (any, error) {
		var req RepositoryManageRequest
		d := json.NewDecoder(bytes.NewReader(payload))
		d.DisallowUnknownFields()
		if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF {
			return nil, control.ErrInvalidArgument
		}
		ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		switch req.Operation {
		case "list":
			if req.ID != "" || req.Owner != "" {
				return nil, control.ErrInvalidArgument
			}
			all, err := s.ListSources(ctx)
			return RepositoryManageResponse{Sources: all}, translateError(err)
		case "delete":
			if !gitrepo.ValidID(req.ID) || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:identity", Owner: req.Owner}) {
				return nil, control.ErrInvalidArgument
			}
			return RepositoryManageResponse{}, translateError(s.DeleteSource(ctx, req.ID, req.Owner))
		default:
			return nil, control.ErrInvalidArgument
		}
	}
}
func (c *Client) RepositoryManage(ctx context.Context, req RepositoryManageRequest) (RepositoryManageResponse, error) {
	var result RepositoryManageResponse
	err := c.wire.Call(ctx, MethodRepositoryManage, req, &result)
	return result, err
}
