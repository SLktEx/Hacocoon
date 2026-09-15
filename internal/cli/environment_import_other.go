//go:build !linux

package cli

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/transfer"
)

func loadEnvironmentImport(context.Context, environmentImportClient, string, string) (environmenttransfer.ImportResult, error) {
	return environmenttransfer.ImportResult{}, core.ErrUnsupported
}
