package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type command func(context.Context, *composition.App, []string) error

func main() {
	ctx := context.Background()
	app, err := composition.Local(ctx)
	if err != nil {
		fail(err)
	}
	commands := map[string]command{
		"init":    initCommand,
		"doctor":  doctorCommand,
		"new":     newCommand,
		"list":    listCommand,
		"status":  statusCommand,
		"start":   startCommand,
		"stop":    stopCommand,
		"rm":      removeCommand,
		"exec":    execCommand,
		"shell":   shellCommand,
		"storage": storageCommand,
	}
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	run, ok := commands[os.Args[1]]
	if !ok {
		usage()
		os.Exit(2)
	}
	if err := run(ctx, app, os.Args[2:]); err != nil {
		fail(err)
	}
}

func initCommand(ctx context.Context, app *composition.App, _ []string) error {
	handle, err := app.Manager.Init(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("initialized storage %s with %s\n", handle.ID, app.Storage.ID())
	return nil
}

func doctorCommand(ctx context.Context, app *composition.App, _ []string) error {
	runtime, runtimeErr := app.Runtime.Probe(ctx)
	storage, storageErr := app.Storage.Probe(ctx)
	fmt.Println("Core")
	fmt.Println("  composition: ok")
	fmt.Printf("Runtime: %s\n", app.Runtime.ID())
	fmt.Printf("  available: %t\n", runtime.Available)
	for _, detail := range runtime.Details {
		fmt.Printf("  %s\n", detail)
	}
	fmt.Printf("Storage: %s\n", app.Storage.ID())
	fmt.Printf("  backend: %s\n", storage.Backend)
	fmt.Printf("  available: %t\n", storage.Available)
	fmt.Printf("  shrink: %t\n", storage.Shrink)
	fmt.Printf("  compact: %t\n", storage.Compact)
	for _, detail := range storage.Details {
		fmt.Printf("  %s\n", detail)
	}
	return errors.Join(runtimeErr, storageErr)
}

func newCommand(ctx context.Context, app *composition.App, args []string) error {
	name := ""
	if len(args) > 0 {
		name = args[0]
	}
	session, err := app.Manager.Create(ctx, core.SessionSpec{Name: name})
	if err != nil {
		return err
	}
	fmt.Printf("%s\t%s\t%s\n", session.ID, session.Name, session.ObservedState)
	return nil
}

func listCommand(ctx context.Context, app *composition.App, _ []string) error {
	sessions, err := app.Manager.List(ctx)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		fmt.Printf("%s\t%s\t%s/%s\n", session.ID, session.Name, session.DesiredState, session.ObservedState)
	}
	return nil
}

func statusCommand(ctx context.Context, app *composition.App, args []string) error {
	session, err := resolveSession(ctx, app, firstArg(args))
	if err != nil {
		return err
	}
	fmt.Printf("id: %s\nname: %s\nruntime: %s (%s)\nstorage: %s (%s)\ndesired: %s\nobserved: %s\n",
		session.ID, session.Name, session.RuntimeModule, session.RuntimeRef, session.StorageModule, session.StorageRef, session.DesiredState, session.ObservedState)
	return nil
}

func startCommand(ctx context.Context, app *composition.App, args []string) error {
	session, err := resolveSession(ctx, app, firstArg(args))
	if err != nil {
		return err
	}
	return app.Manager.Start(ctx, session.ID)
}

func stopCommand(ctx context.Context, app *composition.App, args []string) error {
	session, err := resolveSession(ctx, app, firstArg(args))
	if err != nil {
		return err
	}
	return app.Manager.Stop(ctx, session.ID)
}

func removeCommand(ctx context.Context, app *composition.App, args []string) error {
	session, err := resolveSession(ctx, app, firstArg(args))
	if err != nil {
		return err
	}
	return app.Manager.Remove(ctx, session.ID)
}

func execCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: haco exec <session> -- <command...>")
	}
	session, err := resolveSession(ctx, app, args[0])
	if err != nil {
		return err
	}
	argv := args[1:]
	if argv[0] == "--" {
		argv = argv[1:]
	}
	if len(argv) == 0 {
		return core.ErrInvalidArgument
	}
	result, err := app.Manager.Exec(ctx, session.ID, core.ExecRequest{Argv: argv})
	fmt.Print(result.Stdout)
	fmt.Fprint(os.Stderr, result.Stderr)
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("remote command exited %d", result.ExitCode)
	}
	return nil
}

func shellCommand(ctx context.Context, app *composition.App, args []string) error {
	session, err := resolveSession(ctx, app, firstArg(args))
	if err != nil {
		return err
	}
	_, err = app.Manager.Exec(ctx, session.ID, core.ExecRequest{Argv: []string{"/bin/bash"}, Interactive: true})
	return err
}

func storageCommand(ctx context.Context, app *composition.App, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: haco storage <status|grow|plan-shrink|shrink|compact>")
	}
	handle, err := app.Manager.Init(ctx)
	if err != nil {
		return err
	}
	resizable, ok := app.Storage.(core.ResizableStorage)
	if !ok {
		return core.ErrUnsupported
	}
	actions := map[string]func() error{
		"status": func() error {
			state, err := app.Storage.Inspect(ctx, handle)
			if err != nil {
				return err
			}
			fmt.Printf("backend: %s\nhealthy: %t\nlogical_bytes: %d\nused_bytes: %d\n", state.Backend, state.Healthy, state.LogicalBytes, state.UsedBytes)
			return nil
		},
		"grow": func() error {
			target, err := targetSize(args[1:])
			if err != nil {
				return err
			}
			return resizable.Grow(ctx, handle, target)
		},
		"plan-shrink": func() error {
			target, err := targetSize(args[1:])
			if err != nil {
				return err
			}
			plan, err := resizable.PlanShrink(ctx, handle, target)
			if err != nil {
				return err
			}
			printShrinkPlan(plan)
			return nil
		},
		"shrink": func() error {
			target, err := targetSize(args[1:])
			if err != nil {
				return err
			}
			plan, err := resizable.PlanShrink(ctx, handle, target)
			if err != nil {
				return err
			}
			printShrinkPlan(plan)
			return app.Manager.ShrinkStorage(ctx, handle, plan)
		},
		"compact": func() error { return resizable.Compact(ctx, handle) },
	}
	action, ok := actions[args[0]]
	if !ok {
		return fmt.Errorf("unknown storage action %q", args[0])
	}
	return action()
}

func resolveSession(ctx context.Context, app *composition.App, selector string) (core.Session, error) {
	if selector == "" {
		return core.Session{}, core.ErrInvalidArgument
	}
	sessions, err := app.Manager.List(ctx)
	if err != nil {
		return core.Session{}, err
	}
	for _, session := range sessions {
		if string(session.ID) == selector || session.Name == selector || strings.HasPrefix(string(session.ID), selector) {
			return session, nil
		}
	}
	return core.Session{}, fmt.Errorf("session %q: %w", selector, core.ErrNotFound)
}

func targetSize(args []string) (int64, error) {
	if len(args) != 2 || args[0] != "--to" {
		return 0, fmt.Errorf("expected --to <size>")
	}
	return parseSize(args[1])
}

func parseSize(raw string) (int64, error) {
	text := strings.TrimSpace(strings.ToUpper(raw))
	units := map[string]int64{"K": 1 << 10, "M": 1 << 20, "G": 1 << 30, "T": 1 << 40, "KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40, "KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30, "TIB": 1 << 40}
	for suffix, multiplier := range units {
		if strings.HasSuffix(text, suffix) {
			number := strings.TrimSpace(strings.TrimSuffix(text, suffix))
			value, err := strconv.ParseInt(number, 10, 64)
			if err != nil {
				return 0, err
			}
			return value * multiplier, nil
		}
	}
	return strconv.ParseInt(text, 10, 64)
}

func printShrinkPlan(plan core.ShrinkPlan) {
	fmt.Printf("current_bytes: %d\ntarget_bytes: %d\nminimum_bytes: %d\nsafety_margin_bytes: %d\nfeasible: %t\nrequires_compaction: %t\nreason: %s\n",
		plan.CurrentBytes, plan.TargetBytes, plan.MinimumBytes, plan.SafetyMarginBytes, plan.Feasible, plan.RequiresCompaction, plan.Reason)
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: haco <init|doctor|new|list|status|start|stop|rm|exec|shell|storage>")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "haco:", err)
	os.Exit(1)
}
