//go:build !linux

package basebuild

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
)

func (s *Service) Import(context.Context, ImportRequest, io.Reader, string) (Result, error) {
	return Result{}, core.ErrUnsupported
}
