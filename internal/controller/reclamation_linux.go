//go:build linux && (amd64 || arm64)

package controller

import (
	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
)

func registerReclamation(server *control.Server, app *composition.App) error {
	if err := controlapi.RegisterReclamation(server, app); err != nil {
		return err
	}
	return controlapi.RegisterReclamationTarget(server, app)
}
