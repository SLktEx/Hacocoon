//go:build linux

package controller

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/env/transfer"
	"os"
	"path/filepath"
)

func registerEnvironmentExport(server *control.Server, app *composition.App) error {
	root := os.Getenv("HACO_ROOT")
	if root == "" {
		root = "/var/lib/hacocoon"
	}
	root = filepath.Join(root, "transfers")
	return controlapi.RegisterEnvironmentExport(server, func(ctx context.Context, source string) (environmenttransfer.ExportResult, error) {
		if err := os.MkdirAll(root, 0700); err != nil {
			return environmenttransfer.ExportResult{}, err
		}
		return app.ExportEnvironment(ctx, source, root, controlapi.EnvironmentExportLimit)
	})
}
