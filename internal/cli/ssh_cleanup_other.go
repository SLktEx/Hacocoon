//go:build !linux

package cli

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"io"
)

func cleanupDesktopSSH(context.Context, *controlapi.Client, string, string, io.Writer) {}
