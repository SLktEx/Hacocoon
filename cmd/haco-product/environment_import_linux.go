//go:build linux

package main

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"golang.org/x/sys/unix"
	"io"
	"os"
)

// The selected file is read-only. Never interpret it as a controller-side path.
func loadEnvironmentImport(ctx context.Context, client environmentImportClient, path, name string) (environmenttransfer.ImportResult, error) {
	var result environmenttransfer.ImportResult
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return result, err
	}
	file := os.NewFile(uintptr(fd), "environment import")
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return result, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > environmenttransfer.DefaultPayloadLimit+(512<<10) {
		return result, core.ErrInvalidArgument
	}
	stop := context.AfterFunc(ctx, func() { _ = file.Close() })
	defer stop()
	// Cheap client-side refusal precedes upload; authoritative staging checks again
	// because an input file can change after this inspection.
	if _, err := environmenttransfer.Inspect(io.NewSectionReader(file, 0, info.Size()), environmenttransfer.DefaultPayloadLimit); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	return client.ImportEnvironment(ctx, io.NewSectionReader(file, 0, info.Size()), name)
}
