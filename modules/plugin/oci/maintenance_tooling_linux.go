//go:build linux

package oci

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Prepare returns a private directory containing exactly containerd, ctr and
// nerdctl. The caller must release it after Incus file transfer completes.
func (s *MaintenanceTooling) Prepare(ctx context.Context) (string, func() error, error) {
	if runtime.GOARCH != "amd64" {
		return "", nil, core.ErrUnsupported
	}
	return s.prepare(ctx, maintenanceArchiveSHA256)
}

func toolingPrivate(info os.FileInfo, directory bool) bool {
	st, ok := info.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uint32(os.Geteuid()) && info.Mode().Perm()&0077 == 0 &&
		((directory && info.IsDir()) || (!directory && info.Mode().IsRegular() && st.Nlink == 1))
}

func (s *MaintenanceTooling) prepare(ctx context.Context, expected string) (string, func() error, error) {
	decoded, decodeErr := hex.DecodeString(expected)
	if decodeErr != nil || len(decoded) != sha256.Size || hex.EncodeToString(decoded) != expected {
		return "", nil, core.ErrInvalidArgument
	}
	if !filepath.IsAbs(s.Directory) || filepath.Clean(s.Directory) != s.Directory {
		return "", nil, core.ErrInvalidArgument
	}
	if err := os.MkdirAll(s.Directory, 0700); err != nil {
		return "", nil, err
	}
	info, err := os.Lstat(s.Directory)
	if err != nil || !toolingPrivate(info, true) {
		return "", nil, core.ErrIncompatibleState
	}
	root, err := os.OpenRoot(s.Directory)
	if err != nil {
		return "", nil, err
	}
	defer root.Close()
	observed, err := root.Stat(".")
	if err != nil || !os.SameFile(info, observed) {
		return "", nil, core.ErrIncompatibleState
	}
	lock, err := root.OpenFile("archive.lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return "", nil, err
	}
	defer lock.Close()
	lockInfo, err := lock.Stat()
	if err != nil || !toolingPrivate(lockInfo, false) {
		return "", nil, core.ErrIncompatibleState
	}
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			return "", nil, err
		}
		select {
		case <-ctx.Done():
			return "", nil, ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	archive, err := s.archive(ctx, root, expected)
	if err != nil {
		return "", nil, err
	}
	defer archive.Close()
	directory, err := os.MkdirTemp("", "haco-maintenance-tools-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() error { return os.RemoveAll(directory) }
	if err = extractMaintenanceTools(ctx, archive, directory); err != nil {
		return "", nil, errors.Join(err, cleanup())
	}
	return directory, cleanup, nil
}

func (s *MaintenanceTooling) archive(ctx context.Context, root *os.Root, expected string) (*os.File, error) {
	name := expected + ".tar.gz"
	open := func() (*os.File, error) {
		f, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return nil, err
		}
		info, err := f.Stat()
		if err != nil || !toolingPrivate(info, false) || info.Size() > maxMaintenanceArchive {
			f.Close()
			return nil, core.ErrIncompatibleState
		}
		h := sha256.New()
		if _, err = io.Copy(h, io.LimitReader(f, maxMaintenanceArchive+1)); err != nil {
			f.Close()
			return nil, err
		}
		if hex.EncodeToString(h.Sum(nil)) != expected {
			f.Close()
			return nil, core.ErrIncompatibleState
		}
		if _, err = f.Seek(0, io.SeekStart); err != nil {
			f.Close()
			return nil, err
		}
		return f, nil
	}
	f, err := open()
	if err == nil {
		return f, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	// An exclusive temporary file is never a completed cache entry. Competing
	// downloads may install the same verified bytes, but never partial bytes.
	temporary, err := os.CreateTemp(s.Directory, ".download-")
	if err != nil {
		return nil, err
	}
	temporaryName := filepath.Base(temporary.Name())
	before, err := temporary.Stat()
	defer func() {
		_ = temporary.Close()
		current, e := root.Lstat(temporaryName)
		if e == nil && before != nil && os.SameFile(before, current) {
			_ = root.Remove(temporaryName)
		}
	}()
	observed, observeErr := root.Lstat(temporaryName)
	if err != nil || observeErr != nil || !os.SameFile(before, observed) || !toolingPrivate(before, false) {
		return nil, core.ErrIncompatibleState
	}
	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" || len(via) >= 10 {
				return core.ErrInvalidArgument
			}
			return nil
		}}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, maintenanceArchiveURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download maintenance tools: %w", errors.Join(core.ErrRuntimeUnavailable, ctx.Err()))
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("maintenance tools download status %d: %w", response.StatusCode, core.ErrRuntimeUnavailable)
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(temporary, h), io.LimitReader(response.Body, maxMaintenanceArchive+1))
	if err != nil {
		return nil, err
	}
	if n > maxMaintenanceArchive || hex.EncodeToString(h.Sum(nil)) != expected {
		return nil, core.ErrIncompatibleState
	}
	if err = temporary.Sync(); err != nil {
		return nil, err
	}
	// Link provides no-replace publication; an existing entry is verified below.
	if err = root.Link(temporaryName, name); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	if err = root.Remove(temporaryName); err != nil {
		return nil, err
	}
	if err = temporary.Close(); err != nil {
		return nil, err
	}
	return open()
}

func extractMaintenanceTools(ctx context.Context, file io.Reader, directory string) error {
	zipped, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer zipped.Close()
	// Bound total decompression, including members that are not installed.
	reader := tar.NewReader(io.LimitReader(zipped, 2<<30))
	wanted := map[string]string{"bin/containerd": "containerd", "bin/ctr": "ctr", "bin/nerdctl": "nerdctl"}
	seen := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name, ok := wanted[header.Name]
		if !ok {
			continue
		}
		if seen[name] || header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > 256<<20 {
			return core.ErrIncompatibleState
		}
		output, err := os.OpenFile(filepath.Join(directory, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0500)
		if err != nil {
			return err
		}
		n, copyErr := io.Copy(output, reader)
		closeErr := output.Close()
		if copyErr != nil || closeErr != nil {
			return errors.Join(copyErr, closeErr)
		}
		if n != header.Size {
			return core.ErrIncompatibleState
		}
		seen[name] = true
	}
	if len(seen) != len(wanted) {
		return core.ErrIncompatibleState
	}
	return nil
}
