//go:build !linux

package packer

import (
	"context"

	basebuild "github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func Build(context.Context, basebuild.PackerTemplate, basebuild.ImportRequest, Importer) (basebuild.Result, error) {
	return basebuild.Result{}, core.ErrUnsupported
}
