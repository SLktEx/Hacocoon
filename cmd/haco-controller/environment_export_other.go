//go:build !linux

package main

import (
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
)

func registerEnvironmentExport(*control.Server, *composition.App) error { return nil }
