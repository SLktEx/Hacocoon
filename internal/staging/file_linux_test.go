//go:build linux

package staging

import (
	"context"
	"errors"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"testing"
)

func TestCaptureOwnsBoundedReadOnlyUnnamedInput(t *testing.T) {
	for _, mode := range []string{"ok", "oversize", "failed", "canceled", "public"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			if err := os.Chmod(root, 0700); err != nil {
				t.Fatal(err)
			}
			if mode == "public" {
				if err := os.Chmod(root, 0755); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			file, n, err := Capture(ctx, root, 4, func(out io.Writer) error {
				if mode == "canceled" {
					cancel()
				}
				value := "data"
				if mode == "oversize" {
					value = "extra"
				}
				if _, err := io.WriteString(out, value); err != nil {
					return err
				}
				if mode == "failed" {
					return errors.New("producer failed")
				}
				return nil
			})
			if mode == "ok" {
				if err != nil || file == nil || n != 4 {
					t.Fatal(n, err)
				}
				defer func() { _ = file.Close() }()
				var info unix.Stat_t
				if err := unix.Fstat(int(file.Fd()), &info); err != nil || info.Nlink != 0 {
					t.Fatal("named file", err)
				}
				if _, err := file.WriteAt([]byte("x"), 0); err == nil {
					t.Fatal("writable handle escaped")
				}
				raw, err := io.ReadAll(file)
				if err != nil || string(raw) != "data" {
					t.Fatal(string(raw), err)
				}
			} else if err == nil || file != nil {
				t.Fatal("failed capture exposed file", err)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatal("named capture remnants", err)
			}
		})
	}
}
