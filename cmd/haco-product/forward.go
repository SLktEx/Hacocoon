package main

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

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/streamio"
)

func forwardClientCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	f := flag.NewFlagSet("env tunnel", flag.ContinueOnError)
	configureCLIFlags(f, diagnostic)
	address := f.String("address", "127.0.0.1", cliMessage("forward.address"))
	listen := f.String("listen", "127.0.0.1:0", cliMessage("forward.listen"))
	port := f.Int("target-port", 0, cliMessage("detail.target_port"))
	duration := f.Duration("duration", time.Hour, cliMessage("forward.duration"))
	f.Usage = func() { commandHelp(diagnostic, "env tunnel", cliLanguage()) }
	if err := f.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	host, portText, err := net.SplitHostPort(*listen)
	localPort, portErr := strconv.Atoi(portText)
	ip := net.ParseIP(host)
	if err != nil || portErr != nil || ip == nil || !ip.IsLoopback() || localPort < 0 || localPort > 65535 || len(f.Args()) != 1 || !core.ValidForwardAddress(*address, *port) || *duration < time.Second || *duration > time.Hour {
		f.Usage()
		return 2
	}
	ctx, cancel := context.WithTimeout(ctx, *duration)
	defer cancel()
	client, err := controlapi.NewDefaultClient()
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("error.controller"))
		return 1
	}
	target, err := client.PrepareEnvironmentForward(ctx, f.Args()[0], *address, *port)
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("operation.failed"), err)
		return 1
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("operation.failed"), err)
		return 1
	}
	defer listener.Close()
	if _, err = fmt.Fprintf(out, cliMessage("forward.ready"), listener.Addr().String(), target.Environment, target.Address, target.Port, *duration); err != nil {
		return 1
	}
	var messages sync.Mutex
	err = streamio.Serve(ctx, listener, func(ctx context.Context) (net.Conn, error) { return client.OpenEnvironmentForward(ctx, target) }, func(err error) {
		if err != nil && ctx.Err() == nil {
			messages.Lock()
			defer messages.Unlock()
			fmt.Fprintln(diagnostic, cliMessage("forward.failed"), err)
		}
	})
	if ctx.Err() != nil {
		return 0
	}
	if err != nil {
		fmt.Fprintln(diagnostic, cliMessage("operation.failed"), err)
		return 1
	}
	return 0
}
