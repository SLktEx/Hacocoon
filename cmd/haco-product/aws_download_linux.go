//go:build linux

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

type awsFileWriter struct {
	writer io.Writer
	count  int64
}

func (w *awsFileWriter) Write(p []byte) (int, error) {
	n, err := w.writer.Write(p)
	w.count += int64(n)
	return n, err
}
func saveAWSDownload(ctx context.Context, client awsDownloadClient, spec awsplugin.GetSpec, destination string) (resultErr error) {
	path := filepath.Clean(destination)
	name := filepath.Base(path)
	if name == "." || name == string(filepath.Separator) {
		return fmt.Errorf("select a destination file")
	}
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	parentFD, err := parent.Open(".")
	if err != nil {
		return err
	}
	defer parentFD.Close()
	if info, err := parent.Lstat(name); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("destination must be a regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	temporary := ".haco-download-" + hex.EncodeToString(nonce[:])
	if err := parent.Mkdir(temporary, 0700); err != nil {
		return err
	}
	private, err := parent.OpenRoot(temporary)
	if err != nil {
		return err
	}
	defer private.Close()
	privateFD, err := private.Open(".")
	if err != nil {
		return err
	}
	defer privateFD.Close()
	originalDir, err := privateFD.Stat()
	if err != nil {
		return err
	}
	owner, ok := originalDir.Sys().(*syscall.Stat_t)
	if !ok || owner.Uid != uint32(os.Geteuid()) || originalDir.Mode().Perm() != 0700 {
		return fmt.Errorf("private download directory is not owned and private")
	}
	defer func() {
		if err := private.Remove("content"); err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("download temporary-file cleanup failed: %w", err))
		}
		// Never recursively remove contents or follow a substituted directory.
		current, err := parent.Lstat(temporary)
		if err == nil {
			if os.SameFile(originalDir, current) {
				err = parent.Remove(temporary)
			} else {
				err = fmt.Errorf("private download directory changed; cleanup requires inspection")
			}
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			resultErr = errors.Join(resultErr, fmt.Errorf("download temporary-directory cleanup failed: %w", err))
		}
	}()
	file, err := private.OpenFile("content", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	sink := &awsFileWriter{writer: io.MultiWriter(file, hash)}
	receipt, err := client.DownloadS3(ctx, spec, sink)
	if err != nil {
		return err
	}
	if err := awsplugin.VerifyDownload(receipt, sink.count, hex.EncodeToString(hash.Sum(nil))); err != nil {
		return fmt.Errorf("download receipt did not verify: %w", err)
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Both directory descriptors are pinned. Renaming the destination's parent
	// or the private staging directory cannot redirect this publication.
	if err := unix.Renameat(int(privateFD.Fd()), "content", int(parentFD.Fd()), name); err != nil {
		return err
	}
	if err := parentFD.Sync(); err != nil {
		return fmt.Errorf("download published but directory sync failed: %w", err)
	}
	return nil
}
