//go:build !linux

package main

import (
	"context"
	"io"
)

func cleanupDesktopSSH(context.Context, string, string, io.Writer) {}
