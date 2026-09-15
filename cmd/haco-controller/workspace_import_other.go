//go:build !linux

package main

import (
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/control"
)

func registerWorkspaceImport(*control.Server, *composition.App) error { return nil }
