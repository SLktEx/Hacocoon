//go:build !linux

package cli

import (
	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func readPackerContext(string) (*basebuild.PackerTemplate, error) { return nil, core.ErrUnsupported }
