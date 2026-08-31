package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

type generalControllerClient interface {
	ListBases(context.Context) ([]core.BaseInfo, error)
	InspectBase(context.Context, core.BaseName) (core.BaseInfo, error)
	Run(context.Context, runapp.Spec) (runapp.Result, error)
	StreamEvents(context.Context, int64, func(eventsapp.Event) error) (int64, error)
	RequestCapability(
		context.Context,
		core.CapabilityRequest,
		func(context.Context, core.ApprovalRequest) (bool, error),
	) (core.CapabilityResult, error)
}

type generalControllerFactory func() (generalControllerClient, error)

func init() {
	if isHacocoonLogin(os.Args[0]) || isVersionInvocation(os.Args[1:]) || isHelpInvocation(os.Args[1:]) {
		return
	}
	handled, err := handleGeneralControllerArgs(
		context.Background(),
		os.Args[1:],
		newDefaultGeneralController,
		os.Stdin,
		os.Stdout,
		os.Stderr,
	)
	if !handled {
		return
	}
	if err != nil {
		fail(err)
	}
	os.Exit(0)
}

func newDefaultGeneralController() (generalControllerClient, error) {
	return controlapi.NewDefaultClient()
}

func isGeneralControllerCommand(command string) bool {
	switch command {
	case "base", "run", "events", "capability":
		return true
	default:
		return false
	}
}

func handleGeneralControllerArgs(
	ctx context.Context,
	args []string,
	factory generalControllerFactory,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) (bool, error) {
	if len(args) == 0 || !isGeneralControllerCommand(args[0]) {
		return false, nil
	}
	if factory == nil || stdin == nil || stdout == nil || stderr == nil {
		return true, core.ErrInvalidArgument
	}
	client, err := factory()
	if err != nil {
		return true, err
	}
	switch args[0] {
	case "base":
		return true, generalBaseCommand(ctx, client, args[1:], stdout)
	case "run":
		return true, generalRunCommand(ctx, client, args[1:], stdout, stderr)
	case "events":
		return true, generalEventsCommand(ctx, client, args[1:], stdout)
	case "capability":
		return true, generalCapabilityCommand(ctx, client, args[1:], stdin, stdout, stderr)
	default:
		return false, nil
	}
}

func generalBaseCommand(ctx context.Context, client generalControllerClient, args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco base <list|inspect> ...: %w", core.ErrInvalidArgument)
	}
	switch args[0] {
	case "list":
		jsonOutput := false
		if len(args) == 2 && args[1] == "--json" {
			jsonOutput = true
		} else if len(args) != 1 {
			return fmt.Errorf("usage: haco base list [--json]: %w", core.ErrInvalidArgument)
		}
		bases, err := client.ListBases(ctx)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(out).Encode(bases)
		}
		for _, base := range bases {
			if _, err := fmt.Fprintln(out, base.Name); err != nil {
				return err
			}
		}
		return nil
	case "inspect":
		jsonOutput := false
		if len(args) == 3 && args[2] == "--json" {
			jsonOutput = true
		} else if len(args) != 2 {
			return fmt.Errorf("usage: haco base inspect <base> [--json]: %w", core.ErrInvalidArgument)
		}
		info, err := client.InspectBase(ctx, core.BaseName(args[1]))
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(out).Encode(info)
		}
		_, err = fmt.Fprintf(out, "name: %s\nrevision: %s\n", info.Name, info.Revision)
		return err
	default:
		return fmt.Errorf("unknown base command %q: %w", args[0], core.ErrInvalidArgument)
	}
}

func generalRunCommand(ctx context.Context, client generalControllerClient, args []string, stdout, stderr io.Writer) error {
	spec, jsonOutput, err := parseRunSpec(args)
	if err != nil {
		return err
	}
	result, runErr := client.Run(ctx, spec)
	if jsonOutput {
		if err := json.NewEncoder(stdout).Encode(result); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprint(stdout, result.Execution.Stdout); err != nil {
			return err
		}
		if _, err := fmt.Fprint(stderr, result.Execution.Stderr); err != nil {
			return err
		}
	}
	if runErr != nil {
		return runErr
	}
	if result.Execution.ExitCode > 0 {
		return commandExitError{code: result.Execution.ExitCode}
	}
	return nil
}

func parseGeneralEventsArgs(args []string) (bool, int64, error) {
	jsonOutput := false
	var sinceOffset int64
	offsetSeen := false
	for len(args) > 0 {
		switch args[0] {
		case "--json":
			if jsonOutput {
				return false, 0, core.ErrInvalidArgument
			}
			jsonOutput = true
			args = args[1:]
		case "--since-offset":
			if offsetSeen || len(args) < 2 {
				return false, 0, core.ErrInvalidArgument
			}
			offset, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil || offset < 0 {
				return false, 0, fmt.Errorf("invalid events offset %q: %w", args[1], core.ErrInvalidArgument)
			}
			sinceOffset = offset
			offsetSeen = true
			args = args[2:]
		default:
			return false, 0, fmt.Errorf("usage: haco events [--json] [--since-offset <byte-offset>]: %w", core.ErrInvalidArgument)
		}
	}
	return jsonOutput, sinceOffset, nil
}

func generalEventsCommand(ctx context.Context, client generalControllerClient, args []string, out io.Writer) error {
	jsonOutput, sinceOffset, err := parseGeneralEventsArgs(args)
	if err != nil {
		return err
	}
	var encoder *json.Encoder
	if jsonOutput {
		encoder = json.NewEncoder(out)
	}
	_, err = client.StreamEvents(ctx, sinceOffset, func(event eventsapp.Event) error {
		if encoder != nil {
			return encoder.Encode(event)
		}
		_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", event.Time.UTC().Format("2006-01-02T15:04:05Z07:00"), event.Type, event.Capability, event.Action, event.Decision)
		return err
	})
	return err
}

func generalCapabilityCommand(
	ctx context.Context,
	client generalControllerClient,
	args []string,
	stdin io.Reader,
	stdout io.Writer,
	stderr io.Writer,
) error {
	if len(args) < 3 || args[0] != "request" {
		return fmt.Errorf("usage: haco capability request <capability> <action> [--resource <resource>] [--environment <environment>] [--param <key=value>]...: %w", core.ErrInvalidArgument)
	}
	request, err := parseCapabilityRequest(args[1:])
	if err != nil {
		return err
	}
	approval := capabilityapp.NewStdioApproval(stdin, stderr)
	result, requestErr := client.RequestCapability(ctx, request, approval.Approve)
	if result.Output != "" {
		if _, err := fmt.Fprintln(stdout, result.Output); err != nil {
			return err
		}
	}
	return requestErr
}
