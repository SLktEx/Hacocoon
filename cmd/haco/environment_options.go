package main

import (
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

func parseCreateSpec(args []string) (core.EnvironmentSpec, error) {
	usageError := func() error {
		return fmt.Errorf("usage: haco create [--read-only] [--base <base>] [--cpu <n|unlimited>] [--memory <size|unlimited>] [--pids <n|unlimited>] [--root-size <size|unlimited>] --workspace <path> <environment>: %w", core.ErrInvalidArgument)
	}
	if len(args) < 3 {
		return core.EnvironmentSpec{}, usageError()
	}
	spec := core.EnvironmentSpec{AccessMode: core.WorkspaceReadWrite}
	readOnlySeen := false
	for len(args) > 1 {
		switch args[0] {
		case "--read-only":
			if readOnlySeen {
				return core.EnvironmentSpec{}, usageError()
			}
			readOnlySeen = true
			spec.AccessMode = core.WorkspaceReadOnly
			args = args[1:]
		case "--base":
			if len(args) < 3 || spec.Base != "" {
				return core.EnvironmentSpec{}, usageError()
			}
			spec.Base = core.BaseName(args[1])
			args = args[2:]
		case "--cpu", "--memory", "--pids", "--root-size":
			if len(args) < 3 {
				return core.EnvironmentSpec{}, usageError()
			}
			if err := setResourceOption(&spec.Resources, args[0], args[1]); err != nil {
				return core.EnvironmentSpec{}, err
			}
			args = args[2:]
		case "--workspace":
			if len(args) < 3 || spec.WorkspacePath != "" {
				return core.EnvironmentSpec{}, usageError()
			}
			spec.WorkspacePath = args[1]
			args = args[2:]
		default:
			if len(args) != 1 {
				return core.EnvironmentSpec{}, fmt.Errorf("unknown create option %q: %w", args[0], core.ErrInvalidArgument)
			}
		}
	}
	if len(args) != 1 || spec.WorkspacePath == "" {
		return core.EnvironmentSpec{}, usageError()
	}
	spec.Name = args[0]
	return spec, nil
}

func parseRunSpec(args []string) (runapp.Spec, bool, error) {
	spec := runapp.Spec{AccessMode: core.WorkspaceReadWrite}
	jsonOutput := false
	readOnlySeen := false
	workspaceSeen := false
	separator := -1
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || separator == len(args)-1 {
		return runapp.Spec{}, false, fmt.Errorf("usage: haco run [--read-only] [--cpu <n|unlimited>] [--memory <size|unlimited>] [--pids <n|unlimited>] [--root-size <size|unlimited>] --workspace <path> [--json] -- <command...>: %w", core.ErrInvalidArgument)
	}
	options := args[:separator]
	for len(options) > 0 {
		switch options[0] {
		case "--read-only":
			if readOnlySeen {
				return runapp.Spec{}, false, core.ErrInvalidArgument
			}
			readOnlySeen = true
			spec.AccessMode = core.WorkspaceReadOnly
			options = options[1:]
		case "--cpu", "--memory", "--pids", "--root-size":
			if len(options) < 2 {
				return runapp.Spec{}, false, core.ErrInvalidArgument
			}
			if err := setResourceOption(&spec.Resources, options[0], options[1]); err != nil {
				return runapp.Spec{}, false, err
			}
			options = options[2:]
		case "--workspace":
			if len(options) < 2 || workspaceSeen {
				return runapp.Spec{}, false, core.ErrInvalidArgument
			}
			workspaceSeen = true
			spec.WorkspacePath = options[1]
			options = options[2:]
		case "--json":
			if jsonOutput {
				return runapp.Spec{}, false, core.ErrInvalidArgument
			}
			jsonOutput = true
			options = options[1:]
		default:
			return runapp.Spec{}, false, fmt.Errorf("unknown run option %q: %w", options[0], core.ErrInvalidArgument)
		}
	}
	if !workspaceSeen || strings.TrimSpace(spec.WorkspacePath) == "" {
		return runapp.Spec{}, false, core.ErrInvalidArgument
	}
	spec.Argv = append([]string(nil), args[separator+1:]...)
	return spec, jsonOutput, nil
}
