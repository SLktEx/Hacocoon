//go:build !linux

package cli

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func loadBaseImport(context.Context, baseImportClient, string, basebuild.ImportRequest) (basebuild.Result, error) {
	return basebuild.Result{}, core.ErrUnsupported
}
