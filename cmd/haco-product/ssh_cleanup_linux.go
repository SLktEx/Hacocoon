//go:build linux

package main

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/sshclient"
	"io"
)

func cleanupDesktopSSH(ctx context.Context, client *controlapi.Client, name, grant string, diagnostics io.Writer) {
	d, err := sshclient.ResolveDesktop(ctx)
	if err == nil {
		if grant == "" {
			err = sshclient.CleanupEnvironment(ctx, client, d, name)
		} else {
			err = sshclient.Remove(ctx, d, name, grant)
		}
	}
	if err != nil {
		_, _ = fmt.Fprintln(diagnostics, cliMessage("ssh.cleanup_incomplete"))
	}
}
