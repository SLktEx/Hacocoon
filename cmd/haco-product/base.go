package main

import (
	"context"
	"encoding/json"
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
	if len(args) == 0 || (args[0] != "list" && args[0] != "inspect") || (args[0] == "list" && len(args) != 1) || (args[0] == "inspect" && len(args) != 2) {
		fmt.Fprintln(os.Stderr, "Usage: haco base list | haco base inspect <base>")
		return 2
	}
	c, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var result any
	if args[0] == "list" {
		result, err = c.ListBases(ctx)
	} else {
		result, err = c.InspectBase(ctx, core.BaseName(args[1]))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	if err = json.NewEncoder(os.Stdout).Encode(result); err != nil {
		return 1
	}
	return 0
}
