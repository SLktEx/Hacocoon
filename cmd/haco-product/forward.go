package main

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/clientforward"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"io"
)

func forwardClientCommand(ctx context.Context, args []string, out, diagnostic io.Writer) int {
	return clientforward.Command(ctx, args, out, diagnostic, cliLanguage(), controlapi.NewDefaultClient, func() { commandHelp(diagnostic, "env tunnel", cliLanguage()) })
}
