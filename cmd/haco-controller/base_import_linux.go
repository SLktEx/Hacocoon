//go:build linux

package main

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func registerBaseImport(server *control.Server, app *composition.App) error {
	root := os.Getenv("HACO_ROOT")
	if root == "" {
		root = "/var/lib/hacocoon"
	}
	root = filepath.Join(root, "transfers")
	return controlapi.RegisterBaseImport(server, func(ctx context.Context, r io.Reader, req basebuild.ImportRequest) (basebuild.Result, error) {
		if err := os.MkdirAll(root, 0700); err != nil {
			return basebuild.Result{}, err
		}
		return app.BaseBuild.Import(ctx, req, r, root)
	})
}
