//go:build !linux

package controller

import (
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
)

func registerEnvironmentImport(*control.Server, *composition.App) error { return nil }
