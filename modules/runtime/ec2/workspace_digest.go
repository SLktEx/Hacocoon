package ec2

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const workspaceDigestPrefix = "sha256:"

// digestWorkspace computes a stable streaming identity of the caller-owned
// workspace. It intentionally ignores mtime-only changes, but includes paths,
// entry type, permission/type mode bits, regular-file size/content and symlink
// targets. Unsupported special files fail closed rather than being omitted from
// the conflict boundary.
func digestWorkspace(ctx context.Context, root string) (string, error) {
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace root is not a directory: %w", core.ErrInvalidArgument)
	}

	h := sha256.New()
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "." {
			rel = ""
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if err := writeDigestField(h, "path", filepath.ToSlash(rel)); err != nil {
			return err
		}
		if err := writeDigestField(h, "mode", strconv.FormatUint(uint64(info.Mode()), 8)); err != nil {
			return err
		}

		switch {
		case info.Mode().IsDir():
			return writeDigestField(h, "type", "dir")
		case info.Mode().IsRegular():
			if err := writeDigestField(h, "type", "file"); err != nil {
				return err
			}
			if err := writeDigestField(h, "size", strconv.FormatInt(info.Size(), 10)); err != nil {
				return err
			}
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(h, file)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			_, err = h.Write([]byte{0})
			return err
		case info.Mode()&os.ModeSymlink != 0:
			if err := writeDigestField(h, "type", "symlink"); err != nil {
				return err
			}
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return writeDigestField(h, "target", target)
		default:
			return fmt.Errorf("unsupported workspace entry %q mode=%s: %w", rel, info.Mode(), core.ErrUnsupported)
		}
	}); err != nil {
		return "", err
	}
	return workspaceDigestPrefix + hex.EncodeToString(h.Sum(nil)), nil
}

func writeDigestField(h hash.Hash, name, value string) error {
	if _, err := io.WriteString(h, name); err != nil {
		return err
	}
	if _, err := h.Write([]byte{0}); err != nil {
		return err
	}
	if _, err := io.WriteString(h, value); err != nil {
		return err
	}
	_, err := h.Write([]byte{0})
	return err
}

func validWorkspaceDigest(value string) bool {
	if len(value) != len(workspaceDigestPrefix)+sha256.Size*2 || value[:len(workspaceDigestPrefix)] != workspaceDigestPrefix {
		return false
	}
	decoded, err := hex.DecodeString(value[len(workspaceDigestPrefix):])
	return err == nil && len(decoded) == sha256.Size
}
