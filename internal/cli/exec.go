package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

type execClient interface {
	ExecStream(context.Context, string, core.ProcessRequest, io.Reader, io.Writer, io.Writer) (core.ExecutionResult, error)
}

func runExec(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return execCommand(ctx, controlapi.NewDefaultClient(), args, os.Stdin, os.Stdout, os.Stderr)
}
func execCommand(ctx context.Context, client execClient, args []string, input io.Reader, out, diagnostic io.Writer) int {
	flags := flag.NewFlagSet("haco exec", flag.ContinueOnError)
	flags.SetOutput(diagnostic)
	interactive := flags.Bool("i", false, "Keep stdin open")
	tty := flags.Bool("t", false, "Allocate a terminal")
	work := flags.String("w", "/workspace", "Working directory")
	var options, command []string
	name := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			command = append(command, args[i+1:]...)
			break
		}
		if name != "" && !strings.HasPrefix(arg, "-") {
			command = append(command, args[i:]...)
			break
		}
		if arg == "-it" || arg == "-ti" {
			options = append(options, "-i", "-t")
			continue
		}
		if strings.HasPrefix(arg, "-") {
			options = append(options, arg)
			if arg == "-w" {
				if i+1 == len(args) {
					return 2
				}
				i++
				options = append(options, args[i])
			}
			continue
		}
		name = arg
	}
	if err := flags.Parse(options); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	request := core.ProcessRequest{Argv: command, TTY: *tty, WorkingDirectory: *work}
	if core.ValidateEnvironmentName(name) != nil || core.ValidateProcessRequest(request) != nil {
		fmt.Fprintln(diagnostic, "Usage: haco exec [-i] [-t] [-w DIR] ENV -- COMMAND [ARG...]")
		return 2
	}
	if !*interactive {
		if *tty {
			if fd, ok := input.(interface{ Fd() uintptr }); ok {
				input = closedTerminalInput{Reader: strings.NewReader(""), fd: fd.Fd()}
			}
		} else {
			input = strings.NewReader("")
		}
	}

	result, err := client.ExecStream(ctx, name, request, input, out, diagnostic)
	if errors.Is(err, context.Canceled) {
		return 130
	}
	if err != nil {
		var code interface{ ExitCode() int }
		if errors.As(err, &code) && code.ExitCode() > 0 {
			return code.ExitCode()
		}
		fmt.Fprintln(diagnostic, "haco:", err)
		return 1
	}
	return result.ExitCode
}

type closedTerminalInput struct {
	io.Reader
	fd uintptr
}

func (r closedTerminalInput) Fd() uintptr { return r.fd }
