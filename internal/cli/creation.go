package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/env/creation"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// parseInterspersed preserves values and -- while accepting Docker-style flags
// on either side of the positional Image. It never reparses command strings.
func parseInterspersed(flags *flag.FlagSet, args []string) error {
	var options, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		options = append(options, arg)
		name, _, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		f := flags.Lookup(name)
		if f == nil || hasValue {
			continue
		}
		if boolean, ok := f.Value.(interface{ IsBoolFlag() bool }); ok && boolean.IsBoolFlag() {
			continue
		}
		if i+1 == len(args) {
			return fmt.Errorf("flag needs a value: %s", arg)
		}
		i++
		options = append(options, args[i])
	}
	return flags.Parse(append(append(options, "--"), positional...))
}

type creationClient interface {
	Create(context.Context, creation.Request) (core.Environment, error)
	OpenTarget(context.Context, controlapi.OpenRequest) (core.Environment, error)
}

func runCreate(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	return createCommand(ctx, controlapi.NewDefaultClient(), args, os.Stdout, os.Stderr)
}
func createCommand(ctx context.Context, client creationClient, args []string, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco create", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	name := flags.String("name", "", "Environment name")
	volume := flags.String("volume", "", "Use an existing Volume")
	machine := flags.Bool("json", false, cliMessage("flag.json"))
	if err := parseInterspersed(flags, args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 1 || flags.Arg(0) == "" {
		commandHelp(diagnostic, "create", cliLanguage())
		return 2
	}
	environment, err := client.Create(ctx, creation.Request{Name: *name, Image: core.BaseName(flags.Arg(0)), Volume: *volume})
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	if *machine {
		if writeCLIResult(out, environment, true) != nil {
			return 1
		}
	} else {
		if _, err := fmt.Fprintln(out, environment.Name); err != nil {
			return 1
		}
	}
	return 0
}

func runImage(args []string) int {
	if len(args) == 0 {
		commandHelp(os.Stderr, "image", cliLanguage())
		return 2
	}
	if args[0] == "tag" {
		return runImageCommand("tag", args[1:])
	}
	if args[0] == "rm" && len(args) == 2 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_, err := controlapi.NewDefaultClient().ImageCommand(ctx, controlapi.ImageCommandRequest{Operation: "untag", Source: args[1]})
		if err == nil {
			return 0
		}
		if !isUnsupportedControllerError(err) {
			fmt.Fprintln(os.Stderr, "haco:", err)
			return 1
		}
		return runBase(append([]string{"delete", "--yes"}, args[1:]...))
	}
	if args[0] != "default" {
		args = append([]string(nil), args...)
		if args[0] == "ls" {
			args[0] = "list"
		}
		if args[0] == "rm" {
			args[0] = "delete"
		}
		return runBase(args)
	}
	if len(args) > 2 {
		commandHelp(os.Stderr, "image default", cliLanguage())
		return 2
	}
	var image core.BaseName
	if len(args) == 2 {
		image = core.BaseName(args[1])
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	selected, err := controlapi.NewDefaultClient().DefaultImage(ctx, image)
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	if selected == "" {
		fmt.Fprintln(os.Stderr, "No default Image configured; run haco setup.")
		return 1
	}
	if _, err := fmt.Fprintln(os.Stdout, selected); err != nil {
		return 1
	}
	return 0
}

func runImageCommand(operation string, args []string) int {
	if len(args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: haco image tag SOURCE TARGET | haco commit ENV IMAGE")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	result, err := controlapi.NewDefaultClient().ImageCommand(ctx, controlapi.ImageCommandRequest{Operation: operation, Source: args[0], Target: core.BaseName(args[1])})
	if err != nil {
		fmt.Fprintln(os.Stderr, "haco:", err)
		return 1
	}
	fmt.Fprintln(os.Stdout, result.Name)
	return 0
}
