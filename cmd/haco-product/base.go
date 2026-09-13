package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type baseSwitchClient interface {
	EnvironmentStatus(context.Context, string) (core.EnvironmentStatus, error)
	InspectBase(context.Context, core.BaseName) (core.BaseInfo, error)
	StopEnvironment(context.Context, string) error
	DeleteEnvironment(context.Context, string) error
	CreateEnvironment(context.Context, controlapi.EnvironmentCreateRequest) (core.Environment, error)
}

// Historical asset, not registered by the product CLI. Stage D+ must revisit
// semantics (including persistent resource attachment) before reusing this UX.
// Only canonical lifecycle operations mutate Environment ownership. Each API
// fails closed; a failed replacement leaves the managed Workspace available.
func switchBase(ctx context.Context, c baseSwitchClient, name string, base core.BaseName) (core.Environment, error) {
	status, err := c.EnvironmentStatus(ctx, name)
	if err != nil {
		return core.Environment{}, err
	}
	old := status.Environment
	if !strings.HasPrefix(old.Workspace.Path, "managed:") {
		return core.Environment{}, fmt.Errorf("Base switching requires a managed Workspace: %w", core.ErrInvalidArgument)
	}
	if _, err := c.InspectBase(ctx, base); err != nil {
		return core.Environment{}, err
	}
	if err := c.StopEnvironment(ctx, name); err != nil {
		return core.Environment{}, err
	}
	if err := c.DeleteEnvironment(ctx, name); err != nil {
		return core.Environment{}, err
	}
	result, err := c.CreateEnvironment(ctx, controlapi.EnvironmentCreateRequest{Name: name, WorkspacePath: old.Workspace.Path, AccessMode: old.AccessMode, Base: base, Resources: old.Resources})
	if err != nil {
		return core.Environment{}, fmt.Errorf("Workspace %s retained; inspect 'haco env list', then recreate with 'haco env create --workspace %s --base %s %s': %w", old.Workspace.Path, old.Workspace.Path, base, name, err)
	}
	return result, nil
}

func runBase(args []string) int {
	if len(args) > 0 && args[0] == "delete" {
		return runBaseManage(args)
	}
	if len(args) > 0 && args[0] == "list" {
		for _, arg := range args[1:] {
			if arg == "--all" {
				return runBaseManage(args)
			}
		}
	}
	if len(args) > 0 && args[0] == "build" {
		return runBaseBuild(args[1:])
	}
	clean, jsonOutput, flagErr := splitJSONFlag(args)
	if flagErr != nil {
		fmt.Fprintln(os.Stderr, "haco:", flagErr)
		return 2
	}
	args = clean
	if len(args) == 0 || (args[0] != "list" && args[0] != "inspect") || (args[0] == "list" && len(args) != 1) || (args[0] == "inspect" && len(args) != 2) {
		commandHelp(os.Stderr, "base", cliLanguage())
		return 2
	}
	c, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if args[0] == "list" {
		bases, err := c.ListBases(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "haco:", err)
			return 1
		}
		if jsonOutput {
			if err := writeCLIResult(os.Stdout, bases, true); err != nil {
				return 1
			}
			return 0
		}
		for _, base := range bases {
			fmt.Fprintln(os.Stdout, base.Name)
		}
		return 0
	}
	info, err := c.InspectBase(ctx, core.BaseName(args[1]))
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	if jsonOutput {
		if err := writeCLIResult(os.Stdout, info, true); err != nil {
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stdout, "name: %s\nrevision: %s\n", info.Name, info.Revision)
	return 0
}
