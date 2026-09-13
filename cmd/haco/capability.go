package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func capabilityCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 3 || args[0] != "request" {
		return fmt.Errorf("usage: haco capability request <capability> <action> [--resource <resource>] [--environment <environment>] [--param <key=value>]...: %w", core.ErrInvalidArgument)
	}
	req, err := parseCapabilityRequest(args[1:])
	if err != nil {
		return err
	}
	result, err := app.Capabilities.Request(ctx, req)
	if result.Output != "" {
		fmt.Println(result.Output)
	}
	return err
}

func parseCapabilityRequest(args []string) (core.CapabilityRequest, error) {
	if len(args) < 2 {
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}
	req := core.CapabilityRequest{Capability: args[0], Action: args[1], Parameters: map[string]string{}}
	args = args[2:]
	for len(args) > 0 {
		switch args[0] {
		case "--resource":
			if len(args) < 2 || req.Resource != "" {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			req.Resource = args[1]
			args = args[2:]
		case "--environment":
			if len(args) < 2 || req.Environment != "" {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			req.Environment = args[1]
			args = args[2:]
		case "--param":
			if len(args) < 2 {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			parts := strings.SplitN(args[1], "=", 2)
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			key := strings.TrimSpace(parts[0])
			if _, exists := req.Parameters[key]; exists {
				return core.CapabilityRequest{}, core.ErrInvalidArgument
			}
			req.Parameters[key] = parts[1]
			args = args[2:]
		default:
			return core.CapabilityRequest{}, fmt.Errorf("unknown capability option %q: %w", args[0], core.ErrInvalidArgument)
		}
	}
	return req, nil
}
