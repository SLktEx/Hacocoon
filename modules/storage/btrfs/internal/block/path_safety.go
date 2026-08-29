package block

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ValidateBackingPath enforces the local backing-image trust boundary.
// A backing image is expected at <storage-root>/images/<name>. Both the
// storage root and image directory must be owned by the effective user and
// must not be writable by group/other. Existing backing objects must be
// regular files owned by the effective user and must not be group/other
// writable. Lstat is used deliberately so symlinks fail closed.
func ValidateBackingPath(path string, allowMissing bool) (os.FileInfo, error) {
	if path == "" || filepath.Clean(path) != path {
		return nil, fmt.Errorf("invalid backing image path %q", path)
	}
	imageDir := filepath.Dir(path)
	storageRoot := filepath.Dir(imageDir)
	if err := validateTrustedDirectory(storageRoot, "storage root"); err != nil {
		return nil, err
	}
	if err := validateTrustedDirectory(imageDir, "image directory"); err != nil {
		return nil, err
	}

	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) && allowMissing {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("backing image %q must not be a symbolic link", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("backing image %q must be a regular file (mode %s)", path, info.Mode())
	}
	if err := validateOwnershipAndMode(info, "backing image", path); err != nil {
		return nil, err
	}
	return info, nil
}

// PrepareBackingDirectory creates the expected image directory and then
// validates both it and its storage-root parent with Lstat. Validation happens
// before any backing file is created or resized.
func PrepareBackingDirectory(path string) error {
	if path == "" || filepath.Clean(path) != path {
		return fmt.Errorf("invalid backing image path %q", path)
	}
	imageDir := filepath.Dir(path)
	storageRoot := filepath.Dir(imageDir)
	if err := os.MkdirAll(storageRoot, 0o700); err != nil {
		return err
	}
	if err := validateTrustedDirectory(storageRoot, "storage root"); err != nil {
		return err
	}
	if err := os.Mkdir(imageDir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	return validateTrustedDirectory(imageDir, "image directory")
}

func validateTrustedDirectory(path, kind string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s %q: %w", kind, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s %q must be a real directory, not mode %s", kind, path, info.Mode())
	}
	return validateOwnershipAndMode(info, kind, path)
}

func validateOwnershipAndMode(info os.FileInfo, kind, path string) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("inspect %s ownership for %q", kind, path)
	}
	if int(stat.Uid) != os.Geteuid() {
		return fmt.Errorf("%s %q must be owned by uid %d (got %d)", kind, path, os.Geteuid(), stat.Uid)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s %q must not be group/other writable (mode %04o)", kind, path, info.Mode().Perm())
	}
	return nil
}
