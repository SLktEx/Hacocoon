package controlapi

import (
	"context"
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const MethodBaseImport = "base.import"

func RegisterBaseImport(server *control.Server, receive func(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error)) error {
	if receive == nil {
		return core.ErrInvalidArgument
	}
	return registerImportStream(server, MethodBaseImport, func(payload json.RawMessage) (func(context.Context, io.Reader) (basebuild.Result, error), error) {
		var req basebuild.ImportRequest
		if decodeExportJSON(payload, &req) != nil || req.Validate() != nil {
			return nil, control.ErrInvalidArgument
		}
		return func(ctx context.Context, r io.Reader) (basebuild.Result, error) { return receive(ctx, r, req) }, nil
	}, func(r basebuild.Result) bool { return r.State == "ready" })
}

var importedBaseRevision = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

func validBaseImportResult(r basebuild.Result) bool {
	if r.Base.Name != "" && !basebuild.NamePattern.MatchString(string(r.Base.Name)) || r.Base.Revision != "" && !importedBaseRevision.MatchString(string(r.Base.Revision)) || r.Stage != "" || r.Execution != nil {
		return false
	}
	if r.Builder != "" && (!strings.HasPrefix(r.Builder, "build-") || !core.ValidEnvironmentInstanceID("env-"+strings.TrimPrefix(r.Builder, "build-"))) {
		return false
	}
	switch r.State {
	case "", "failed", "ready", "cleanup-required", "publication-unconfirmed":
		return true
	default:
		return false
	}
}
func (c *Client) ImportBase(ctx context.Context, source io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
	if source == nil || req.Validate() != nil {
		return basebuild.Result{}, core.ErrInvalidArgument
	}
	return uploadInput(ctx, c, source, MethodBaseImport, req, validBaseImportResult, func(r basebuild.Result) bool {
		return r.State == "ready" && r.Base.Name == req.Name && importedBaseRevision.MatchString(string(r.Base.Revision)) && r.Builder == ""
	})
}
