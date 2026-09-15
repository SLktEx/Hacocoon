//go:build !linux

package main

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func loadBaseImport(context.Context, baseImportClient, string, basebuild.ImportRequest) (basebuild.Result, error) {
	return basebuild.Result{}, core.ErrUnsupported
}
