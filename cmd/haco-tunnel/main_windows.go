// haco-tunnel is the Windows client companion for ordinary env tunnel commands.
// It receives no provider access; all operations use the existing WSL controller.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/SLktEx/Hacocoon/internal/clientforward"
	"github.com/SLktEx/Hacocoon/internal/cliui"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/wsllaunch"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	code := run(ctx, os.Args[1:])
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string) int {
	language := cliui.Resolve(os.Getenv)
	usage := func() { writeHelp(os.Stderr, language) }
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		writeHelp(os.Stdout, language)
		return 0
	}
	if len(args) < 3 || args[0] != "--distribution" {
		usage()
		return 2
	}
	dial, err := wsllaunch.ControlDialer(args[1])
	if err != nil {
		usage()
		return 2
	}
	if len(args) == 3 && (args[2] == "--help" || args[2] == "-h") {
		writeHelp(os.Stdout, language)
		return 0
	}
	connect := func() (*controlapi.Client, error) { return controlapi.NewClientWithDialer(dial) }
	code := clientforward.Command(ctx, args[2:], os.Stdout, os.Stderr, language, connect, usage)
	if code == 1 {
		fmt.Fprintln(os.Stderr, language.Format("forward.windows_next", args[1]))
	}
	return code
}

func writeHelp(out io.Writer, language cliui.Language) {
	options := append([]cliui.HelpField{{Syntax: "--distribution <name>", Message: "forward.distribution"}}, clientforward.HelpOptions()...)
	cliui.WriteCommandHelp(out, "haco-tunnel.exe", "", []cliui.CommandHelp{{
		Path: "", Syntax: "--distribution <name> --target-port <port> [--listen <address:port>] [--address <IP>] [--duration <duration>] <environment>",
		Message: "forward.command", Arguments: []cliui.HelpField{{Syntax: "<environment>", Message: "detail.env"}}, Options: options,
		Example: "haco-tunnel.exe --distribution hacocoon --target-port 8080 demo",
	}}, language)
}
