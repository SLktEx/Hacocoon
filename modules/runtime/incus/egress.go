package incus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const egressProxyDevice = "haco-egress"
const egressProxyListen = "tcp:127.0.0.1:3128"

var environmentRuntimeRefPattern = regexp.MustCompile(`^haco-[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)

// EnsureEgressProxy exposes only the host-owned egress broker on instance
// loopback. The normal NIC remains under the default-deny ACL; this proxy
// device is the sole application-layer route for HTTP(S) egress.
func (r *Runtime) EnsureEgressProxy(ctx context.Context, ref, socketPath string) error {
	if r == nil || r.runner == nil || !environmentRuntimeRefPattern.MatchString(ref) {
		return core.ErrInvalidArgument
	}
	if err := validateEgressSocket(socketPath); err != nil {
		return err
	}

	devices, err := r.runner.Run(ctx, "incus", "config", "device", "list", ref, "--project", r.project)
	if err != nil {
		return fmt.Errorf("list Incus devices before egress proxy setup: %w", err)
	}
	if listedDevice(devices.Stdout, egressProxyDevice) {
		if _, err := r.runner.Run(ctx, "incus", "config", "device", "remove", ref, egressProxyDevice, "--project", r.project); err != nil {
			return fmt.Errorf("remove stale Incus egress proxy: %w", err)
		}
	}

	connect := "unix:" + socketPath
	if _, err := r.runner.Run(ctx, "incus", "config", "device", "add", ref, egressProxyDevice, "proxy",
		"bind=instance",
		"listen="+egressProxyListen,
		"connect="+connect,
		"--project", r.project,
	); err != nil {
		return fmt.Errorf("add Incus egress proxy: %w", err)
	}

	want := map[string]string{
		"type":    "proxy",
		"bind":    "instance",
		"listen":  egressProxyListen,
		"connect": connect,
	}
	for key, expected := range want {
		got, err := r.runner.Run(ctx, "incus", "config", "device", "get", ref, egressProxyDevice, key, "--project", r.project)
		if err != nil {
			return fmt.Errorf("verify Incus egress proxy %s: %w", key, err)
		}
		if strings.TrimSpace(got.Stdout) != expected {
			return fmt.Errorf("Incus egress proxy %s drifted from managed value: %w", key, core.ErrIncompatibleState)
		}
	}
	return nil
}

func validateEgressSocket(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsAny(path, "\r\n\x00") {
		return core.ErrInvalidArgument
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("inspect egress broker socket: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("egress broker path is not a trusted Unix socket: %w", core.ErrIncompatibleState)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("egress broker socket is accessible outside its owner: %w", core.ErrIncompatibleState)
	}
	return nil
}

func listedDevice(raw, name string) bool {
	for _, line := range strings.Split(raw, "\n") {
		if strings.TrimSpace(line) == name {
			return true
		}
	}
	return false
}
