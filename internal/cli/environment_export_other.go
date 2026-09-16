//go:build !linux

package cli

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func saveEnvironmentExport(context.Context, environmentExportClient, string, string) (controlapi.EnvironmentExportResult, error) {
	return controlapi.EnvironmentExportResult{}, core.ErrUnsupported
}
