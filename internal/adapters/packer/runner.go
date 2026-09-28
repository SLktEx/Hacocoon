// Package packer builds native transport artifacts in the trusted logical Host.
// The controller never imports this package or evaluates Packer input.
package packer

import (
	"context"
	_ "embed"
	"io"

	basebuild "github.com/SLktEx/Hacocoon/internal/base/build"
)

//go:embed build.py
var program string

type Importer interface {
	ImportBase(context.Context, io.Reader, basebuild.ImportRequest) (basebuild.Result, error)
}
