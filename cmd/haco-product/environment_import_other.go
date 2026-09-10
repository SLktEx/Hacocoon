//go:build !linux

package main

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
)

func loadEnvironmentImport(context.Context, environmentImportClient, string, string) (environmenttransfer.ImportResult, error) {
	return environmenttransfer.ImportResult{}, core.ErrUnsupported
}
