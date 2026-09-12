package main

import (
	"context"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type command func(context.Context, *composition.App, []string) error

func main() {
	if isHacocoonLogin(os.Args[0]) {
		if err := runHacocoonLogin(os.Args[1:]); err != nil {
			fail(err)
		}
		return
	}

	ctx := context.Background()
	app, err := composition.Local(ctx)
	if err != nil {
		fail(err)
	}
	if err := dispatch(ctx, app, os.Args[1:]); err != nil {
		fail(err)
	}
}

func dispatch(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		usage()
		return core.ErrInvalidArgument
	}
	commands := map[string]command{
		"create":      createCommand,
		"base":        baseCommand,
		"plugin":      pluginCommand,
		"run":         runCommand,
		"events":      eventsCommand,
		"capability":  capabilityCommand,
		"egress":      egressCommand,
		"status":      statusCommand,
		"connections": connectionsCommand,
		"forward":     forwardCommand,
		"unforward":   unforwardCommand,
		"ssh":         sshCommand,
		"exec":        execCommand,
		"shell":       shellCommand,
		"delete":      deleteCommand,
		"doctor":      doctorCommand,
		"host":        hostCommand,
	}
	run, ok := commands[args[0]]
	if !ok {
		usage()
		return fmt.Errorf("unknown command %q: %w", args[0], core.ErrInvalidArgument)
	}
	return run(ctx, app, args[1:])
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: haco <create|base|plugin|run|events|egress|status|connections|forward|unforward|ssh|capability|exec|shell|delete|doctor|host>")
}
