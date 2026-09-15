package main

import (
	"context"
	"io"

	"github.com/SLktEx/Hacocoon/internal/clientforward"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func forwardClientCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	return clientforward.DesktopCommand(ctx, args, out, diagnostic, cliLanguage(), controlapi.NewDefaultClient, func() { commandHelp(diagnostic, "env tunnel", cliLanguage()) })
}
