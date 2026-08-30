package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	seedbuildapp "github.com/SLktEx/Hacocoon/internal/seedbuild"
)

func ociSeedPinCommand(ctx context.Context, app *composition.App, args []string, remove bool) error {
	if app == nil || app.Seeds == nil {
		return fmt.Errorf("OCI Seed builder is unavailable for the configured plugin: %w", core.ErrRuntimeUnavailable)
	}
	if len(args) == 0 {
		return fmt.Errorf("Seed pin command requires reference@sha256:...: %w", core.ErrInvalidArgument)
	}
	identity := args[0]
	base, jsonOutput, err := parseOCISeedBaseOptions(args[1:])
	if err != nil {
		return err
	}
	if remove {
		removed, err := app.Seeds.Unpin(ctx, base, identity)
		if err != nil {
			return err
		}
		result := struct {
			Identity string `json:"identity"`
			Removed  bool   `json:"removed"`
		}{Identity: identity, Removed: removed}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(result)
		}
		state := "not-pinned"
		if removed {
			state = "unpinned"
		}
		fmt.Printf("%s: %s\n", state, identity)
		return nil
	}
	pin, err := app.Seeds.Pin(ctx, base, identity)
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(pin)
	}
	fmt.Printf("pinned: %s\nbase: %s\npinned-at: %s\n", pin.Image.String(), pin.Base, pin.PinnedAt.UTC().Format("2006-01-02T15:04:05Z"))
	return nil
}

func ociSeedPinsCommand(ctx context.Context, app *composition.App, args []string) error {
	if app == nil || app.Seeds == nil {
		return fmt.Errorf("OCI Seed builder is unavailable for the configured plugin: %w", core.ErrRuntimeUnavailable)
	}
	base, jsonOutput, err := parseOCISeedBaseOptions(args)
	if err != nil {
		return err
	}
	pins, err := app.Seeds.Pins(ctx, base)
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(pins)
	}
	for _, pin := range pins {
		fmt.Printf("%s\t%s\t%s\n", pin.Base, pin.Image.String(), pin.PinnedAt.UTC().Format("2006-01-02T15:04:05Z"))
	}
	return nil
}

func ociSeedMaintenanceCommand(ctx context.Context, app *composition.App, args []string, recoverBuilders bool) error {
	if app == nil || app.Seeds == nil {
		return fmt.Errorf("OCI Seed builder is unavailable for the configured plugin: %w", core.ErrRuntimeUnavailable)
	}
	jsonOutput, err := parseOCISeedJSONOnly(args)
	if err != nil {
		return err
	}
	var report seedbuildapp.MaintenanceReport
	if recoverBuilders {
		report, err = app.Seeds.Recover(ctx)
	} else {
		report, err = app.Seeds.GC(ctx)
	}
	if jsonOutput {
		if encodeErr := json.NewEncoder(os.Stdout).Encode(report); encodeErr != nil {
			return encodeErr
		}
	} else {
		printOCISeedMaintenanceReport(report)
	}
	return err
}

func printOCISeedMaintenanceReport(report seedbuildapp.MaintenanceReport) {
	for _, builder := range report.DeletedBuilders {
		fmt.Printf("builder: %s\tdeleted\n", builder)
	}
	for _, revision := range report.DeletedImages {
		fmt.Printf("image: %s\tdeleted\n", revision)
	}
	for revision, reason := range report.RetainedImages {
		fmt.Printf("image: %s\tretained\t%s\n", revision, reason)
	}
	for target, reason := range report.Failures {
		fmt.Fprintf(os.Stderr, "maintenance: %s\tfailed\t%s\n", target, reason)
	}
}

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
