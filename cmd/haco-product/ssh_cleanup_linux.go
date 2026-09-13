//go:build linux

package main

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/sshclient"
	"io"
)

func cleanupDesktopSSH(ctx context.Context, name, grant string, diagnostics io.Writer) {
	d, err := sshclient.ResolveDesktop(ctx)
	if err == nil {
		err = sshclient.Remove(ctx, d, name, grant)
	}
	if err != nil {
		fmt.Fprintln(diagnostics, "haco: connection removed; managed desktop entry could not be cleaned")
	}
}
