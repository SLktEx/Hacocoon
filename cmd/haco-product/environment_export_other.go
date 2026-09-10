//go:build !linux

package main

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func saveEnvironmentExport(context.Context, environmentExportClient, string, string) (controlapi.EnvironmentExportResult, error) {
	return controlapi.EnvironmentExportResult{}, core.ErrUnsupported
}
