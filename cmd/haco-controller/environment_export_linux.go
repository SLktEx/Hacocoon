//go:build linux

package main

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
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
