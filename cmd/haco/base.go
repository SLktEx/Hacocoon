package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func baseCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco base <list|inspect> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "list":
		return baseListCommand(ctx, app, args[1:])
	case "inspect":
		return baseInspectCommand(ctx, app, args[1:])
	default:
		return fmt.Errorf("unknown base command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func baseListCommand(ctx context.Context, app *composition.App, args []string) error {
	jsonOutput := false
	if len(args) == 1 && args[0] == "--json" {
		jsonOutput = true
	} else if len(args) != 0 {
		return fmt.Errorf("usage: haco base list [--json]: %w", core.ErrInvalidArgument)
	}
	bases, err := app.Bases.ListBases(ctx)
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(bases)
	}
	for _, base := range bases {
		fmt.Println(base.Name)
	}
	return nil
}

func baseInspectCommand(ctx context.Context, app *composition.App, args []string) error {
	jsonOutput := false
	if len(args) == 2 && args[1] == "--json" {
		jsonOutput = true
	} else if len(args) != 1 {
		return fmt.Errorf("usage: haco base inspect <base> [--json]: %w", core.ErrInvalidArgument)
	}
	info, err := app.Bases.InspectBase(ctx, core.BaseName(args[0]))
	if err != nil {
		return err
	}
	if jsonOutput {
		return json.NewEncoder(os.Stdout).Encode(info)
	}
	fmt.Printf("name: %s\nrevision: %s\n", info.Name, info.Revision)
	return nil
}
