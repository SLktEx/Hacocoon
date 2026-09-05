package storagepriv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/host"
)

const DefaultHelperPath = "/usr/local/libexec/hacocoon/haco-storage-helper"
const defaultSudoPath = "/usr/bin/sudo"
const managedBtrfsMountOptions = "compress=zstd:3,noatime,nodiscard"

type SudoRunner struct {
	root           string
	direct         host.Runner
	helperPath     string
	sudoPath       string
	euid           func() int
	validateHelper func(string) error
}

// NewSudoRunner keeps ordinary storage inspection/file operations in the
// caller process while routing only the explicitly allowlisted operations that
// require Host privilege through haco-storage-helper.
func NewSudoRunner(root string, direct host.Runner) (*SudoRunner, error) {
	if direct == nil {
		return nil, fmt.Errorf("storage privilege runner requires a direct Host runner")
	}
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, fmt.Errorf("storage privilege root must be an absolute clean path: %q", root)
	}
	helperPath := strings.TrimSpace(os.Getenv("HACO_STORAGE_HELPER"))
	if helperPath == "" {
		helperPath = DefaultHelperPath
	}
	return &SudoRunner{
		root:           root,
		direct:         direct,
		helperPath:     helperPath,
		sudoPath:       defaultSudoPath,
		euid:           os.Geteuid,
		validateHelper: validateTrustedHelperExecutable,
	}, nil
}

func (r *SudoRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	op, opArgs, privileged, err := translatePrivilegedCommand(name, args)
	if err != nil {
		return host.Result{ExitCode: -1}, err
	}
	if !privileged {
		return r.direct.Run(ctx, name, args...)
	}
	if err := r.validateHelper(r.helperPath); err != nil {
		return host.Result{ExitCode: -1}, fmt.Errorf("validate Hacocoon storage helper: %w", err)
	}

	helperArgs := append([]string{"--root", r.root, op}, opArgs...)
	if r.euid() == 0 {
		return r.direct.Run(ctx, r.helperPath, helperArgs...)
	}
	if err := validateTrustedExecutable(r.sudoPath); err != nil {
		return host.Result{ExitCode: -1}, fmt.Errorf("privileged storage operation %s requires trusted sudo at %s: %w", op, r.sudoPath, err)
	}
	sudoArgs := append([]string{"--", r.helperPath}, helperArgs...)
	return r.direct.Run(ctx, r.sudoPath, sudoArgs...)
}

func translatePrivilegedCommand(name string, args []string) (string, []string, bool, error) {
	switch filepath.Base(name) {
	case "losetup":
		if len(args) == 1 && args[0] == "--version" {
			return "", nil, false, nil
		}
		switch {
		case len(args) == 2 && args[0] == "-j":
			return "loop-find", []string{args[1]}, true, nil
		case len(args) == 3 && args[0] == "--find" && args[1] == "--show":
			return "loop-attach", []string{args[2]}, true, nil
		case len(args) == 2 && args[0] == "-d":
			return "loop-detach", []string{args[1]}, true, nil
		case len(args) == 2 && args[0] == "-c":
			return "loop-rescan", []string{args[1]}, true, nil
		default:
			return "", nil, false, fmt.Errorf("unsupported privileged losetup arguments: %q", args)
		}
	case "blkid":
		if len(args) == 5 && args[0] == "-o" && args[1] == "value" && args[2] == "-s" && args[3] == "TYPE" {
			return "fs-type", []string{args[4]}, true, nil
		}
		return "", nil, false, fmt.Errorf("unsupported privileged blkid arguments: %q", args)
	case "mkfs.btrfs":
		if len(args) == 2 && args[0] == "-f" {
			return "fs-format-btrfs", []string{args[1]}, true, nil
		}
		return "", nil, false, fmt.Errorf("unsupported privileged mkfs.btrfs arguments: %q", args)
	case "mount":
		if len(args) == 4 && args[2] == "-o" && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
			switch args[3] {
			case managedBtrfsMountOptions:
				return "mount-btrfs", []string{args[0], args[1]}, true, nil
			case "remount," + managedBtrfsMountOptions:
				return "remount-btrfs", []string{args[0], args[1]}, true, nil
			}
		}
		return "", nil, false, fmt.Errorf("unsupported privileged mount arguments: %q", args)
	case "umount":
		if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
			return "unmount-btrfs", []string{args[0]}, true, nil
		}
		return "", nil, false, fmt.Errorf("unsupported privileged umount arguments: %q", args)
	case "btrfs":
		if len(args) == 1 && args[0] == "version" {
			return "", nil, false, nil
		}
		switch {
		case len(args) == 4 && args[0] == "filesystem" && args[1] == "usage" && args[2] == "-b":
			return "btrfs-usage", []string{args[3]}, true, nil
		case len(args) == 4 && args[0] == "filesystem" && args[1] == "resize":
			return "btrfs-resize", []string{args[3], args[2]}, true, nil
		case len(args) == 3 && args[0] == "inspect-internal" && args[1] == "min-dev-size":
			return "btrfs-min-size", []string{args[2]}, true, nil
		case len(args) == 5 && args[0] == "balance" && args[1] == "start" && strings.HasPrefix(args[2], "-dusage=") && strings.HasPrefix(args[3], "-musage="):
			dataFilter := strings.TrimPrefix(args[2], "-d")
			metaFilter := strings.TrimPrefix(args[3], "-m")
			if dataFilter != metaFilter {
				return "", nil, false, fmt.Errorf("mismatched btrfs balance filters")
			}
			return "btrfs-balance", []string{args[4], dataFilter}, true, nil
		default:
			return "", nil, false, fmt.Errorf("unsupported privileged btrfs arguments: %q", args)
		}
	case "fstrim":
		if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
			return "trim", []string{args[0]}, true, nil
		}
		return "", nil, false, fmt.Errorf("unsupported privileged fstrim arguments: %q", args)
	default:
		return "", nil, false, nil
	}
}

// validateTrustedExecutable is for fixed OS-provided executables. Distribution
// paths such as /usr/bin/sudo may be symlinks; resolve them first and validate
// the canonical target and its parent chain as root-owned and non-writable.
func validateTrustedExecutable(path string) error {
	if err := validateExecutablePathSyntax(path); err != nil {
		return err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(resolved) || filepath.Clean(resolved) != resolved {
		return fmt.Errorf("resolved executable path must be absolute and clean: %q", resolved)
	}
	return validateRootOwnedExecutable(resolved, false)
}

// validateTrustedHelperExecutable is intentionally stricter than the OS-tool
// validator: Hacocoon's installed privileged helper must itself not be a
// symlink, even when the host distribution uses symlinks for system tools.
func validateTrustedHelperExecutable(path string) error {
	if err := validateExecutablePathSyntax(path); err != nil {
		return err
	}
	return validateRootOwnedExecutable(path, true)
}

func validateExecutablePathSyntax(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("executable path must be absolute and clean: %q", path)
	}
	return nil
}

func validateRootOwnedExecutable(path string, rejectSymlink bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if rejectSymlink && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("executable %q must not be a symbolic link", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("executable %q must be a regular file", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != 0 {
		return fmt.Errorf("executable %q must be owned by root", path)
	}
	if info.Mode().Perm()&0o022 != 0 || info.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("executable %q must be executable and not group/other writable", path)
	}
	return validateRootOwnedParentChain(filepath.Dir(path))
}

func validateRootOwnedParentChain(path string) error {
	for dir := path; ; dir = filepath.Dir(dir) {
		dirInfo, err := os.Lstat(dir)
		if err != nil {
			return err
		}
		dirStat, ok := dirInfo.Sys().(*syscall.Stat_t)
		if dirInfo.Mode()&os.ModeSymlink != 0 || !dirInfo.IsDir() || !ok || dirStat.Uid != 0 || dirInfo.Mode().Perm()&0o022 != 0 {
			return fmt.Errorf("executable parent %q must be a root-owned non-writable real directory", dir)
		}
		if dir == string(filepath.Separator) {
			break
		}
	}
	return nil
}
