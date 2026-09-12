package main

import (
	"context"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func doctorCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: haco doctor: %w", core.ErrInvalidArgument)
	}
	caps, err := app.Runtime.Probe(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Hacocoon Secure Workspace Runtime")
	fmt.Printf("Incus available: %t\n", caps.Available)
	for _, detail := range caps.Details {
		fmt.Printf("  %s\n", detail)
	}
	if !caps.Available {
		return core.ErrRuntimeUnavailable
	}
	return nil
}
