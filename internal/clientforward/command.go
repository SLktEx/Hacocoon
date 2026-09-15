// Package clientforward owns the common client-side TCP tunnel command.
package clientforward

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/streamio"
)

func Command(ctx context.Context, args []string, out, diagnostic io.Writer, language cliui.Language, connect func() (*controlapi.Client, error), usage func()) int {
	return command(ctx, args, out, diagnostic, language, connect, usage, runPrepared)
}

type prepared struct {
	Target   core.EnvironmentTCPForward `json:"target"`
	Listen   string                     `json:"listen"`
	Duration time.Duration              `json:"duration"`
	Language cliui.Language             `json:"language"`
}

type preparedRunner func(context.Context, *controlapi.Client, prepared, io.Writer, io.Writer) int

func validListener(address string) bool {
	host, portText, err := net.SplitHostPort(address)
	port, portErr := strconv.Atoi(portText)
	ip := net.ParseIP(host)
	return err == nil && portErr == nil && ip != nil && ip.IsLoopback() && port >= 0 && port <= 65535
}

func command(ctx context.Context, args []string, out, diagnostic io.Writer, language cliui.Language, connect func() (*controlapi.Client, error), usage func(), run preparedRunner) int {
	message := language.Format
	f := flag.NewFlagSet("env tunnel", flag.ContinueOnError)
	f.SetOutput(diagnostic)
	address := f.String("address", "127.0.0.1", message("forward.address"))
	listen := f.String("listen", "127.0.0.1:0", message("forward.listen"))
	port := f.Int("target-port", 0, message("detail.target_port"))
	duration := f.Duration("duration", time.Hour, message("forward.duration"))
	if usage != nil {
		f.Usage = usage
	}
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if !validListener(*listen) || len(f.Args()) != 1 || !core.ValidForwardAddress(*address, *port) || *duration < time.Second || *duration > time.Hour {
		f.Usage()
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()
	client, err := connect()
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, message("error.controller"))
		return 1
	}
	target, err := client.PrepareEnvironmentForward(ctx, f.Args()[0], *address, *port)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, message("operation.failed"), err)
		return 1
	}
	return run(ctx, client, prepared{Target: target, Listen: *listen, Duration: *duration, Language: language}, out, diagnostic)
}

func runPrepared(ctx context.Context, client *controlapi.Client, request prepared, out, diagnostic io.Writer) int {
	message := request.Language.Format
	target := request.Target
	if ctx.Err() != nil {
		return 0
	}
	listener, err := net.Listen("tcp", request.Listen)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, message("operation.failed"), err)
		return 1
	}
	defer func() { _ = listener.Close() }()
	if _, err = fmt.Fprint(out, message("forward.ready", listener.Addr().String(), target.Environment, target.Address, target.Port, request.Duration)); err != nil {
		return 1
	}
	var messages sync.Mutex
	err = streamio.Serve(ctx, listener, func(ctx context.Context) (net.Conn, error) { return client.OpenEnvironmentForward(ctx, target) }, func(err error) {
		if err != nil && ctx.Err() == nil {
			messages.Lock()
			defer messages.Unlock()
			_, _ = fmt.Fprintln(diagnostic, message("forward.failed"), err)
		}
	})
	if ctx.Err() != nil {
		return 0
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, message("operation.failed"), err)
		return 1
	}
	return 0
}
