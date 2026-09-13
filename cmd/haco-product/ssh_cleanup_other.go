//go:build !linux

package main

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"io"
)

func cleanupDesktopSSH(context.Context, *controlapi.Client, string, string, io.Writer) {}
