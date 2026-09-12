package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	gitcapapp "github.com/SLktEx/Hacocoon/internal/gitcap"
)

func pluginCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco plugin <git|oci> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "git":
		return gitPluginCommand(ctx, app, args[1:])
	case "oci":
		return ociPluginCommand(ctx, app, args[1:])
	default:
		return fmt.Errorf("unknown plugin %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func gitPluginCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: haco plugin git <fetch|push> ...: %w", core.ErrInvalidArgument)
	}

	switch args[0] {
	case "fetch":
		spec, err := parseGitFetchSpec(args[1:])
		if err != nil {
			return err
		}
		result, err := app.Git.Fetch(ctx, spec)
		if result.Output != "" {
			fmt.Println(result.Output)
		}
		return err
	case "push":
		spec, err := parseGitPushSpec(args[1:])
		if err != nil {
			return err
		}
		result, err := app.Git.Push(ctx, spec)
		if result.Output != "" {
			fmt.Println(result.Output)
		}
		return err
	default:
		return fmt.Errorf("usage: haco plugin git <fetch|push> ...: %w", core.ErrInvalidArgument)
	}
}

func parseGitFetchSpec(args []string) (gitcapapp.FetchSpec, error) {
	if len(args) < 1 {
		return gitcapapp.FetchSpec{}, core.ErrInvalidArgument
	}
	spec := gitcapapp.FetchSpec{Environment: args[0], Remote: "origin"}
	args = args[1:]
	seenRemote := false
	for len(args) > 0 {
		switch args[0] {
		case "--remote":
			if len(args) < 2 || seenRemote {
				return gitcapapp.FetchSpec{}, core.ErrInvalidArgument
			}
			spec.Remote, seenRemote, args = args[1], true, args[2:]
		default:
			return gitcapapp.FetchSpec{}, fmt.Errorf("unknown git fetch option %q: %w", args[0], core.ErrInvalidArgument)
		}
	}
	if strings.TrimSpace(spec.Environment) == "" || strings.TrimSpace(spec.Remote) == "" {
		return gitcapapp.FetchSpec{}, core.ErrInvalidArgument
	}
	return spec, nil
}

func parseGitPushSpec(args []string) (gitcapapp.PushSpec, error) {
	if len(args) < 3 {
		return gitcapapp.PushSpec{}, core.ErrInvalidArgument
	}
	spec := gitcapapp.PushSpec{Environment: args[0], Remote: "origin", Source: "HEAD"}
	args = args[1:]
	seenBranch := false
	seenSource := false
	seenRemote := false
	for len(args) > 0 {
		switch args[0] {
		case "--branch":
			if len(args) < 2 || seenBranch {
				return gitcapapp.PushSpec{}, core.ErrInvalidArgument
			}
			spec.Branch, seenBranch, args = args[1], true, args[2:]
		case "--source":
			if len(args) < 2 || seenSource {
				return gitcapapp.PushSpec{}, core.ErrInvalidArgument
			}
			spec.Source, seenSource, args = args[1], true, args[2:]
		case "--remote":
			if len(args) < 2 || seenRemote {
				return gitcapapp.PushSpec{}, core.ErrInvalidArgument
			}
			spec.Remote, seenRemote, args = args[1], true, args[2:]
		case "--force":
			if spec.Force {
				return gitcapapp.PushSpec{}, core.ErrInvalidArgument
			}
			spec.Force, args = true, args[1:]
		default:
			return gitcapapp.PushSpec{}, fmt.Errorf("unknown git push option %q: %w", args[0], core.ErrInvalidArgument)
		}
	}
	if strings.TrimSpace(spec.Environment) == "" || !seenBranch || strings.TrimSpace(spec.Branch) == "" {
		return gitcapapp.PushSpec{}, core.ErrInvalidArgument
	}
	return spec, nil
}
