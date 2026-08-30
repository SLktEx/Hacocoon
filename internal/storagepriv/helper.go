package storagepriv

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/host"
)

const helperValidationExit = 125

var helperStorageIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var loopDevicePattern = regexp.MustCompile(`^/dev/loop[0-9]+$`)

type Helper struct {
	runner      host.Runner
	euid        func() int
	getenv      func(string) string
	resolveTool func(string) (string, error)
}

func NewHelper(runner host.Runner) *Helper {
	return &Helper{
		runner:      runner,
		euid:        os.Geteuid,
		getenv:      os.Getenv,
		resolveTool: resolveSystemTool,
	}
}

// Execute runs one typed privileged storage operation. The helper deliberately
// does not accept an executable name or arbitrary argv from its caller.
func (h *Helper) Execute(ctx context.Context, args []string) host.Result {
	if h == nil || h.runner == nil {
		return helperFailure(fmt.Errorf("storage helper is not configured"))
	}
	if h.euid() != 0 {
		return helperFailure(fmt.Errorf("storage helper must run with effective uid 0"))
	}
	if len(args) < 3 || args[0] != "--root" {
		return helperFailure(fmt.Errorf("usage: haco-storage-helper --root <haco-root> <operation> [arguments]"))
	}
	root := args[1]
	operation := args[2]
	opArgs := args[3:]
	callerUID, err := h.callerUID()
	if err != nil {
		return helperFailure(err)
	}
	if err := validateManagedRoot(root, callerUID); err != nil {
		return helperFailure(err)
	}

	switch operation {
	case "loop-find":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("loop-find requires one backing path"))
		}
		if _, err := validateManagedBacking(root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "losetup", "-j", opArgs[0])
	case "loop-attach":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("loop-attach requires one backing path"))
		}
		if _, err := validateManagedBacking(root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "losetup", "--find", "--show", "--nooverlap", opArgs[0])
	case "loop-detach":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("loop-detach requires one loop device"))
		}
		if _, _, err := h.validateManagedLoop(ctx, root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "losetup", "-d", opArgs[0])
	case "loop-rescan":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("loop-rescan requires one loop device"))
		}
		if _, _, err := h.validateManagedLoop(ctx, root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "losetup", "-c", opArgs[0])
	case "fs-type":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("fs-type requires one loop device"))
		}
		if _, _, err := h.validateManagedLoop(ctx, root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "blkid", "-o", "value", "-s", "TYPE", opArgs[0])
	case "fs-format-btrfs":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("fs-format-btrfs requires one loop device"))
		}
		return h.formatBtrfs(ctx, root, opArgs[0], callerUID)
	case "mount-btrfs":
		if len(opArgs) != 2 {
			return helperFailure(fmt.Errorf("mount-btrfs requires device and mountpoint"))
		}
		return h.mountBtrfs(ctx, root, opArgs[0], opArgs[1], callerUID, false)
	case "remount-btrfs":
		if len(opArgs) != 2 {
			return helperFailure(fmt.Errorf("remount-btrfs requires device and mountpoint"))
		}
		return h.mountBtrfs(ctx, root, opArgs[0], opArgs[1], callerUID, true)
	case "unmount-btrfs":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("unmount-btrfs requires one mountpoint"))
		}
		return h.unmountBtrfs(ctx, root, opArgs[0], callerUID)
	case "btrfs-usage":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("btrfs-usage requires one mountpoint"))
		}
		if _, _, err := h.validateManagedMount(ctx, root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "btrfs", "filesystem", "usage", "-b", opArgs[0])
	case "btrfs-resize":
		if len(opArgs) != 2 {
			return helperFailure(fmt.Errorf("btrfs-resize requires mountpoint and target"))
		}
		if _, _, err := h.validateManagedMount(ctx, root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		if err := validateResizeTarget(opArgs[1]); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "btrfs", "filesystem", "resize", opArgs[1], opArgs[0])
	case "btrfs-min-size":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("btrfs-min-size requires one mountpoint"))
		}
		if _, _, err := h.validateManagedMount(ctx, root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "btrfs", "inspect-internal", "min-dev-size", opArgs[0])
	case "btrfs-balance":
		if len(opArgs) != 2 {
			return helperFailure(fmt.Errorf("btrfs-balance requires mountpoint and filter"))
		}
		if _, _, err := h.validateManagedMount(ctx, root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		if opArgs[1] != "usage=25" && opArgs[1] != "usage=50" && opArgs[1] != "usage=75" {
			return helperFailure(fmt.Errorf("unsupported btrfs balance filter %q", opArgs[1]))
		}
		return h.run(ctx, "btrfs", "balance", "start", "-d"+opArgs[1], "-m"+opArgs[1], opArgs[0])
	case "trim":
		if len(opArgs) != 1 {
			return helperFailure(fmt.Errorf("trim requires one mountpoint"))
		}
		if _, _, err := h.validateManagedMount(ctx, root, opArgs[0], callerUID); err != nil {
			return helperFailure(err)
		}
		return h.run(ctx, "fstrim", opArgs[0])
	default:
		return helperFailure(fmt.Errorf("unsupported privileged storage operation %q", operation))
	}
}

func (h *Helper) callerUID() (uint32, error) {
	raw := strings.TrimSpace(h.getenv("SUDO_UID"))
	if raw == "" {
		if h.euid() == 0 {
			return 0, nil
		}
		return 0, fmt.Errorf("SUDO_UID is required for a non-root storage caller")
	}
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid SUDO_UID %q", raw)
	}
	return uint32(value), nil
}

func (h *Helper) run(ctx context.Context, tool string, args ...string) host.Result {
	path, err := h.resolveTool(tool)
	if err != nil {
		return helperFailure(err)
	}
	result, runErr := h.runner.Run(ctx, path, args...)
	if runErr != nil && result.ExitCode == 0 {
		result.ExitCode = helperValidationExit
	}
	return result
}

func (h *Helper) formatBtrfs(ctx context.Context, root, device string, callerUID uint32) host.Result {
	if _, _, err := h.validateManagedLoop(ctx, root, device, callerUID); err != nil {
		return helperFailure(err)
	}
	probe := h.run(ctx, "blkid", "-o", "value", "-s", "TYPE", device)
	switch {
	case probe.ExitCode == 0 && strings.TrimSpace(probe.Stdout) != "":
		return helperFailure(fmt.Errorf("refuse to format %s: existing filesystem type is %q", device, strings.TrimSpace(probe.Stdout)))
	case probe.ExitCode != 2:
		return probe
	}
	// Re-validate the loop mapping immediately before the destructive operation.
	if _, _, err := h.validateManagedLoop(ctx, root, device, callerUID); err != nil {
		return helperFailure(err)
	}
	return h.run(ctx, "mkfs.btrfs", "-f", device)
}

func (h *Helper) mountBtrfs(ctx context.Context, root, device, mountpoint string, callerUID uint32, remount bool) host.Result {
	deviceID, _, err := h.validateManagedLoop(ctx, root, device, callerUID)
	if err != nil {
		return helperFailure(err)
	}
	mountID, err := validateManagedMountPath(root, mountpoint, callerUID)
	if err != nil {
		return helperFailure(err)
	}
	if deviceID != mountID {
		return helperFailure(fmt.Errorf("loop device storage id %q does not match mountpoint storage id %q", deviceID, mountID))
	}

	if remount {
		mountedID, mountedDevice, err := h.validateManagedMount(ctx, root, mountpoint, callerUID)
		if err != nil {
			return helperFailure(err)
		}
		if mountedID != mountID || mountedDevice != device {
			return helperFailure(fmt.Errorf("refuse remount: %s is not mounted from %s", mountpoint, device))
		}
		return h.run(ctx, "mount", device, mountpoint, "-o", "remount,compress=zstd:3")
	}

	find := h.run(ctx, "findmnt", "-rn", "-o", "SOURCE", "--target", mountpoint)
	if find.ExitCode == 0 && strings.TrimSpace(find.Stdout) != "" {
		mountedDevice := strings.TrimSpace(find.Stdout)
		mountedID, _, err := h.validateManagedLoop(ctx, root, mountedDevice, callerUID)
		if err != nil || mountedID != mountID || mountedDevice != device {
			return helperFailure(fmt.Errorf("refuse mount: %s is already mounted from an unexpected source", mountpoint))
		}
		return host.Result{ExitCode: 0}
	}
	if find.ExitCode != 1 {
		return find
	}
	return h.run(ctx, "mount", device, mountpoint, "-o", "compress=zstd:3")
}

func (h *Helper) unmountBtrfs(ctx context.Context, root, mountpoint string, callerUID uint32) host.Result {
	mountID, err := validateManagedMountPath(root, mountpoint, callerUID)
	if err != nil {
		return helperFailure(err)
	}
	find := h.run(ctx, "findmnt", "-rn", "-o", "SOURCE", "--target", mountpoint)
	if find.ExitCode == 1 || strings.TrimSpace(find.Stdout) == "" {
		return host.Result{ExitCode: 0}
	}
	if find.ExitCode != 0 {
		return find
	}
	device := strings.TrimSpace(find.Stdout)
	deviceID, _, err := h.validateManagedLoop(ctx, root, device, callerUID)
	if err != nil || deviceID != mountID {
		return helperFailure(fmt.Errorf("refuse unmount: %s is not backed by its Hacocoon-managed loop device", mountpoint))
	}
	return h.run(ctx, "umount", mountpoint)
}

func (h *Helper) validateManagedMount(ctx context.Context, root, mountpoint string, callerUID uint32) (string, string, error) {
	mountID, err := validateManagedMountPath(root, mountpoint, callerUID)
	if err != nil {
		return "", "", err
	}
	find := h.run(ctx, "findmnt", "-rn", "-o", "SOURCE", "--target", mountpoint)
	if find.ExitCode != 0 || strings.TrimSpace(find.Stdout) == "" {
		return "", "", fmt.Errorf("managed mountpoint %q is not mounted", mountpoint)
	}
	device := strings.TrimSpace(find.Stdout)
	deviceID, _, err := h.validateManagedLoop(ctx, root, device, callerUID)
	if err != nil {
		return "", "", err
	}
	if deviceID != mountID {
		return "", "", fmt.Errorf("mountpoint storage id %q does not match loop storage id %q", mountID, deviceID)
	}
	return mountID, device, nil
}

func (h *Helper) validateManagedLoop(ctx context.Context, root, device string, callerUID uint32) (string, string, error) {
	if !loopDevicePattern.MatchString(device) {
		return "", "", fmt.Errorf("invalid loop device %q", device)
	}
	result := h.run(ctx, "losetup", "--json", "--list", "--output", "NAME,BACK-FILE", device)
	if result.ExitCode != 0 {
		return "", "", fmt.Errorf("inspect loop device %s: %s", device, strings.TrimSpace(result.Stderr))
	}
	var listing struct {
		LoopDevices []struct {
			Name     string `json:"name"`
			BackFile string `json:"back-file"`
		} `json:"loopdevices"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &listing); err != nil {
		return "", "", fmt.Errorf("decode loop device identity for %s: %w", device, err)
	}
	if len(listing.LoopDevices) != 1 || listing.LoopDevices[0].Name != device || listing.LoopDevices[0].BackFile == "" {
		return "", "", fmt.Errorf("loop device %s has no unique backing file", device)
	}
	backing := filepath.Clean(listing.LoopDevices[0].BackFile)
	id, err := validateManagedBacking(root, backing, callerUID)
	if err != nil {
		return "", "", fmt.Errorf("loop device %s is not Hacocoon-managed: %w", device, err)
	}
	return id, backing, nil
}

func validateManagedRoot(root string, callerUID uint32) error {
	if root == "" || !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return fmt.Errorf("Hacocoon root must be an absolute clean path: %q", root)
	}
	return validateCallerDirectory(root, callerUID, "Hacocoon root")
}

func validateManagedBacking(root, path string, callerUID uint32) (string, error) {
	if err := validateManagedRoot(root, callerUID); err != nil {
		return "", err
	}
	images := filepath.Join(root, "images")
	if err := validateCallerDirectory(images, callerUID, "Hacocoon image directory"); err != nil {
		return "", err
	}
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Dir(path) != images {
		return "", fmt.Errorf("backing image %q is outside %s", path, images)
	}
	base := filepath.Base(path)
	if !strings.HasSuffix(base, ".raw") {
		return "", fmt.Errorf("backing image %q must use the .raw suffix", path)
	}
	id := strings.TrimSuffix(base, ".raw")
	if !helperStorageIDPattern.MatchString(id) || filepath.Join(images, id+".raw") != path {
		return "", fmt.Errorf("invalid managed backing image %q", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("backing image %q must be a regular non-symlink file", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != callerUID {
		return "", fmt.Errorf("backing image %q must be owned by invoking uid %d", path, callerUID)
	}
	if stat.Nlink != 1 {
		return "", fmt.Errorf("backing image %q must not have hard links", path)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return "", fmt.Errorf("backing image %q must not be group/other writable", path)
	}
	return id, nil
}

func validateManagedMountPath(root, mountpoint string, callerUID uint32) (string, error) {
	if err := validateManagedRoot(root, callerUID); err != nil {
		return "", err
	}
	mounts := filepath.Join(root, "mounts")
	if err := validateCallerDirectory(mounts, callerUID, "Hacocoon mounts directory"); err != nil {
		return "", err
	}
	if mountpoint == "" || !filepath.IsAbs(mountpoint) || filepath.Clean(mountpoint) != mountpoint || filepath.Dir(mountpoint) != mounts {
		return "", fmt.Errorf("mountpoint %q is outside %s", mountpoint, mounts)
	}
	id := filepath.Base(mountpoint)
	if !helperStorageIDPattern.MatchString(id) || filepath.Join(mounts, id) != mountpoint {
		return "", fmt.Errorf("invalid managed mountpoint %q", mountpoint)
	}
	info, err := os.Lstat(mountpoint)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "", fmt.Errorf("mountpoint %q must be a real directory", mountpoint)
	}
	return id, nil
}

func validateCallerDirectory(path string, callerUID uint32, kind string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect %s %q: %w", kind, path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s %q must be a real directory", kind, path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != callerUID {
		return fmt.Errorf("%s %q must be owned by invoking uid %d", kind, path, callerUID)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("%s %q must not be group/other writable", kind, path)
	}
	return nil
}

func validateResizeTarget(target string) error {
	if target == "max" {
		return nil
	}
	value, err := strconv.ParseInt(target, 10, 64)
	if err != nil || value <= 0 || strconv.FormatInt(value, 10) != target {
		return fmt.Errorf("invalid btrfs resize target %q", target)
	}
	return nil
}

func helperFailure(err error) host.Result {
	return host.Result{
		ExitCode: helperValidationExit,
		Stderr:   "haco-storage-helper: " + err.Error() + "\n",
	}
}

func resolveSystemTool(name string) (string, error) {
	candidates := map[string][]string{
		"losetup":    {"/usr/sbin/losetup", "/sbin/losetup", "/usr/bin/losetup"},
		"blkid":      {"/usr/sbin/blkid", "/sbin/blkid", "/usr/bin/blkid"},
		"mkfs.btrfs": {"/usr/bin/mkfs.btrfs", "/usr/sbin/mkfs.btrfs", "/sbin/mkfs.btrfs"},
		"findmnt":    {"/usr/bin/findmnt", "/bin/findmnt"},
		"mount":      {"/usr/bin/mount", "/bin/mount"},
		"umount":     {"/usr/bin/umount", "/bin/umount"},
		"btrfs":      {"/usr/bin/btrfs", "/usr/sbin/btrfs", "/sbin/btrfs"},
		"fstrim":     {"/usr/sbin/fstrim", "/sbin/fstrim", "/usr/bin/fstrim"},
	}
	paths, ok := candidates[name]
	if !ok {
		return "", fmt.Errorf("unsupported system storage tool %q", name)
	}
	for _, path := range paths {
		if err := validateTrustedExecutable(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("trusted system storage tool %q was not found", name)
}
