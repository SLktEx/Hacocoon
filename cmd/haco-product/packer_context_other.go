//go:build !linux

package main

import (
	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func readPackerContext(string) (*basebuild.PackerTemplate, error) { return nil, core.ErrUnsupported }
