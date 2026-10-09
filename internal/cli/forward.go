package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"github.com/SLktEx/Hacocoon/internal/client/forward"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

func forwardClientCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	return forwardClientCommandUsing(ctx, args, out, diagnostic, clientforward.DesktopCommand)
}

type forwardCommandRunner func(context.Context, []string, io.Writer, io.Writer, cliui.Language, *controlapi.Client, func()) int

func forwardClientCommandUsing(ctx context.Context, args []string, out, diagnostic io.Writer, run forwardCommandRunner) int {
	logger, err := logging.NewFromEnv(diagnostic)
	if err != nil {
		_, _ = fmt.Fprintln(diagnostic, cliMessage("error.logging"))
		return 2
	}
	return run(logging.WithLogger(ctx, logger), args, out, diagnostic, cliLanguage(), controlapi.NewDefaultClient(), func() { commandHelp(diagnostic, "env tunnel", cliLanguage()) })
}
