//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/environmenttransfer"
	"golang.org/x/sys/unix"
)

type exportFileWriter struct {
	writer    io.Writer
	count     int64
	remaining int64
}

func (w *exportFileWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, environmenttransfer.ErrInvalidBundle
	}
	n, err := w.writer.Write(p)
	w.count += int64(n)
	w.remaining -= int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	return n, err
}
func saveEnvironmentExport(ctx context.Context, client environmentExportClient, source, destination string) (result controlapi.EnvironmentExportResult, err error) {
	path := filepath.Clean(destination)
	name := filepath.Base(path)
	if destination == "" || name == "." || name == ".." || name == string(filepath.Separator) {
		return result, errors.New("select a destination file")
	}
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return result, err
	}
	defer parent.Close()
	directory, err := parent.Open(".")
	if err != nil {
		return result, err
	}
	defer directory.Close()
	if _, err := parent.Lstat(name); err == nil {
		return result, os.ErrExist
	} else if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	// No named partial file, pathname-based cleanup or overwrite race. Unsupported
	// filesystems fail explicitly before the controller starts capture.
	fd, err := unix.Openat(int(directory.Fd()), ".", unix.O_TMPFILE|unix.O_RDWR|unix.O_CLOEXEC, 0600)
	if err != nil {
		return result, fmt.Errorf("destination requires anonymous-file support: %w", err)
	}
	file := os.NewFile(uintptr(fd), "environment export")
	defer func() { err = errors.Join(err, file.Close()) }()
	hash := sha256.New()
	sink := &exportFileWriter{writer: io.MultiWriter(file, hash), remaining: controlapi.EnvironmentExportLimit + (512 << 10)}
	result, err = client.ExportEnvironment(ctx, source, sink)
	if err != nil {
		return result, err
	}
	if result.TemporarySnapshot != "" || result.Bytes != sink.count || result.SHA256 != hex.EncodeToString(hash.Sum(nil)) {
		return result, environmenttransfer.ErrInvalidBundle
	}
	manifest, err := environmenttransfer.Inspect(io.NewSectionReader(file, 0, sink.count), controlapi.EnvironmentExportLimit)
	if err != nil {
		return result, err
	}
	if manifest.Source != source {
		return result, environmenttransfer.ErrInvalidBundle
	}
	if err := file.Sync(); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	// open(2) documents this proc-fd link form for an unprivileged O_TMPFILE owner.
	// Link the live inode into the pinned directory; linkat never replaces a name.
	// https://man7.org/linux/man-pages/man2/open.2.html
	if err := unix.Linkat(unix.AT_FDCWD, fmt.Sprintf("/proc/self/fd/%d", fd), int(directory.Fd()), name, unix.AT_SYMLINK_FOLLOW); err != nil {
		return result, err
	}
	if err := directory.Sync(); err != nil {
		return result, fmt.Errorf("export published but directory sync failed: %w", err)
	}
	return result, nil
}
