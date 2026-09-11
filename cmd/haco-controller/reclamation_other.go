//go:build !linux || (!amd64 && !arm64)

package main

import (
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
)

func registerReclamation(server *control.Server, app *composition.App) error { return nil }
