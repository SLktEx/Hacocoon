package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func ociImageReenableCommand(ctx context.Context, app *composition.App, args []string) error {
	if app == nil || app.OCI == nil || len(args) == 0 || len(args) > 2 {
		return fmt.Errorf("usage: haco plugin oci image reenable <reference@sha256:...> [--json]: %w", core.ErrInvalidArgument)
	}
	jsonOutput := false
	if len(args) == 2 {
		if args[1] != "--json" {
			return fmt.Errorf("unknown OCI image reenable option %q: %w", args[1], core.ErrInvalidArgument)
		}
		jsonOutput = true
	}
	report, err := app.OCI.ReenableImage(ctx, args[0])
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(report)
	}
	state := "not-deleted"
	if report.Removed {
		state = "re-enabled"
	}
	fmt.Printf("image: %s@%s\nstate: %s\n", report.Reference, report.Digest, state)
	return nil
}
