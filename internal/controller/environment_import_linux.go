//go:build linux

package controller

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/env/transfer"
	"io"
	"os"
	"path/filepath"
)

func registerEnvironmentImport(server *control.Server, app *composition.App) error {
	root := os.Getenv("HACO_ROOT")
	if root == "" {
		root = "/var/lib/hacocoon"
	}
	root = filepath.Join(root, "transfers")
	return controlapi.RegisterEnvironmentImport(server, func(ctx context.Context, r io.Reader, name string) (environmenttransfer.ImportResult, error) {
		if err := os.MkdirAll(root, 0700); err != nil {
			return environmenttransfer.ImportResult{}, err
		}
		return app.ImportEnvironment(ctx, r, name, root)
	})
}
