//go:build linux

package main

import (
	"context"
	"io"
	"os"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/core"
	"golang.org/x/sys/unix"
)

func loadBaseImport(ctx context.Context, client baseImportClient, path string, req basebuild.ImportRequest) (basebuild.Result, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return basebuild.Result{}, err
	}
	file := os.NewFile(uintptr(fd), "Base archive input")
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return basebuild.Result{}, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > basebuild.MaxArchiveBytes {
		return basebuild.Result{}, core.ErrInvalidArgument
	}
	stop := context.AfterFunc(ctx, func() { _ = file.Close() })
	defer stop()
	return client.ImportBase(ctx, io.NewSectionReader(file, 0, info.Size()), req)
}
