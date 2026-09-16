package cli

import (
	"context"
	"io"

	"github.com/SLktEx/Hacocoon/internal/client/forward"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
)

func forwardClientCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	return clientforward.DesktopCommand(ctx, args, out, diagnostic, cliLanguage(), controlapi.NewDefaultClient(), func() { commandHelp(diagnostic, "env tunnel", cliLanguage()) })
}
