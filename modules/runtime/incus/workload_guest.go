package incus

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	workloadBrokerRoot              = "/run/hacocoon/workloads"
	environmentWorkloadSocket       = "/run/hacocoon/workload.sock"
	environmentWorkloadDevice       = "haco-workload"
	environmentNerdctlBinaryPath    = "/usr/local/libexec/nerdctl"
	environmentNerdctlLauncherPath  = "/usr/local/bin/nerdctl"
	environmentLegacyNerdctlPath    = "/usr/local/libexec/nerdctl-containerd"
	environmentNerdctlLauncherMark  = "# hacocoon-incus-nerdctl"
	workloadShimBinaryEnvironment   = "HACO_WORKLOAD_SHIM_BINARY"
)

var legacyOCIUnits = []string{
	"hacocoon-docker.socket",
	"hacocoon-docker.service",
	"docker.socket",
	"docker.service",
	"containerd.service",
}

// WorkloadBrokerSocketPath returns the Physical-Host Unix socket dedicated to
// one Environment. The path itself is part of the authority boundary: the
// guest receives only a proxy to this socket, never the Incus daemon socket or
// the full Hacocoon controller socket.
func WorkloadBrokerSocketPath(environment string) (string, error) {
	if err := validateWorkloadToken("environment", environment); err != nil {
		return "", err
	}
	return filepath.Join(workloadBrokerRoot, environment+".sock"), nil
}

// EnsureEnvironmentWorkloadIntegration installs/reconciles the Incus-native
// nerdctl path for a managed Environment. It is safe to call repeatedly and is
// also used to migrate already-existing Environments when the controller
// starts after an upgrade.
func (r *Runtime) EnsureEnvironmentWorkloadIntegration(ctx context.Context, environment, ref string) error {
	if r == nil || r.runner == nil {
		return fmt.Errorf("Incus workload integration is unavailable")
	}
	if err := validateWorkloadToken("environment", environment); err != nil {
		return err
	}
	expectedRef := "haco-" + environment
	if ref != expectedRef {
		return fmt.Errorf("Environment %q runtime ref %q does not match %q", environment, ref, expectedRef)
	}
	if err := r.verifyManagedEnvironment(ctx, environment); err != nil {
		return err
	}

	source, digest, available, err := workloadShimSource()
	if err != nil {
		return err
	}
	// Unit tests and source-tree development can run without a packaged
	// companion haco-host binary. Installed releases place haco-host next to the
	// controller/haco binary; HACO_WORKLOAD_SHIM_BINARY is the explicit test/dev
	// override.
	if !available {
		return nil
	}
	if err := r.ensureEnvironmentWorkloadDevice(ctx, environment, ref); err != nil {
		return err
	}
	if err := r.provisionEnvironmentNerdctl(ctx, environment, ref, source, digest); err != nil {
		return err
	}
	if err := r.disableLegacyOCIUnits(ctx, ref); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) ensureEnvironmentWorkloadDevice(ctx context.Context, environment, ref string) error {
	hostSocket, err := WorkloadBrokerSocketPath(environment)
	if err != nil {
		return err
	}
	result, err := r.runner.Run(ctx, "incus", "config", "device", "list", ref, "--project", r.project)
	if err != nil {
		return fmt.Errorf("list Environment workload devices: %w", err)
	}
	found := false
	for _, name := range strings.Fields(result.Stdout) {
		if name == environmentWorkloadDevice {
			found = true
			break
		}
	}
	if !found {
		if _, err := r.runner.Run(ctx, "incus", "config", "device", "add",
			ref, environmentWorkloadDevice, "proxy",
			"bind=instance",
			"listen=unix:"+environmentWorkloadSocket,
			"connect=unix:"+hostSocket,
			"mode=0666",
			"uid=0",
			"gid=0",
			"--project", r.project,
		); err != nil {
			return fmt.Errorf("add Environment workload broker proxy: %w", err)
		}
	}
	return r.verifyEnvironmentWorkloadDevice(ctx, ref, hostSocket)
}

func (r *Runtime) verifyEnvironmentWorkloadDevice(ctx context.Context, ref, hostSocket string) error {
	expected := []struct {
		key   string
		value string
	}{
		{"type", "proxy"},
		{"bind", "instance"},
		{"listen", "unix:" + environmentWorkloadSocket},
		{"connect", "unix:" + hostSocket},
		{"mode", "0666"},
		{"uid", "0"},
		{"gid", "0"},
		{"nat", ""},
		{"proxy_protocol", ""},
		{"security.uid", ""},
		{"security.gid", ""},
	}
	for _, item := range expected {
		result, err := r.runner.Run(ctx, "incus", "config", "device", "get",
			ref, environmentWorkloadDevice, item.key, "--project", r.project)
		if err != nil {
			return fmt.Errorf("inspect Environment workload proxy %s: %w", item.key, err)
		}
		if strings.TrimSpace(result.Stdout) != item.value {
			return fmt.Errorf("Environment workload proxy %s mismatch: got %q want %q",
				item.key, strings.TrimSpace(result.Stdout), item.value)
		}
	}
	return nil
}

func (r *Runtime) provisionEnvironmentNerdctl(ctx context.Context, environment, ref, source, digest string) error {
	if err := r.backupLegacyEnvironmentNerdctl(ctx, ref); err != nil {
		return err
	}
	if _, err := r.runner.Run(ctx, "incus", "file", "push", source,
		ref+environmentNerdctlBinaryPath,
		"--project", r.project,
		"--create-dirs",
		"--uid", "0", "--gid", "0", "--mode", "0755",
	); err != nil {
		return fmt.Errorf("install Environment Incus nerdctl client: %w", err)
	}
	if err := r.verifyEnvironmentNerdctlBinary(ctx, ref, digest); err != nil {
		return err
	}

	launcher, cleanup, err := writeEnvironmentNerdctlLauncher(environment)
	if err != nil {
		return err
	}
	defer cleanup()
	if _, err := r.runner.Run(ctx, "incus", "file", "push", launcher,
		ref+environmentNerdctlLauncherPath,
		"--project", r.project,
		"--create-dirs",
		"--uid", "0", "--gid", "0", "--mode", "0755",
	); err != nil {
		return fmt.Errorf("install Environment nerdctl launcher: %w", err)
	}
	if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
		"--", "grep", "-Fxq", environmentNerdctlLauncherMark, environmentNerdctlLauncherPath); err != nil {
		return fmt.Errorf("verify Environment nerdctl launcher: %w", err)
	}
	return nil
}

func (r *Runtime) backupLegacyEnvironmentNerdctl(ctx context.Context, ref string) error {
	// A launcher carrying our marker is already managed and can be replaced
	// idempotently.
	if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
		"--", "grep", "-Fxq", environmentNerdctlLauncherMark, environmentNerdctlLauncherPath); err == nil {
		return nil
	}
	if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
		"--", "test", "-e", environmentNerdctlLauncherPath); err != nil {
		return nil
	}
	if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
		"--", "test", "-f", environmentNerdctlLauncherPath, "-a", "-x", environmentNerdctlLauncherPath); err != nil {
		return fmt.Errorf("existing Environment nerdctl path is not a regular executable; refusing takeover")
	}
	if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
		"--", "test", "!", "-e", environmentLegacyNerdctlPath); err != nil {
		return fmt.Errorf("legacy nerdctl backup path already exists; refusing ambiguous takeover")
	}
	if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
		"--", "mv", environmentNerdctlLauncherPath, environmentLegacyNerdctlPath); err != nil {
		return fmt.Errorf("preserve legacy containerd nerdctl binary: %w", err)
	}
	return nil
}

func (r *Runtime) verifyEnvironmentNerdctlBinary(ctx context.Context, ref, digest string) error {
	result, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
		"--", "sha256sum", environmentNerdctlBinaryPath)
	if err != nil {
		return fmt.Errorf("hash Environment Incus nerdctl client: %w", err)
	}
	fields := strings.Fields(result.Stdout)
	if len(fields) < 1 || fields[0] != digest {
		return fmt.Errorf("Environment Incus nerdctl client digest mismatch")
	}
	stat, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
		"--", "stat", "-c", "%a:%u:%g", environmentNerdctlBinaryPath)
	if err != nil {
		return fmt.Errorf("inspect Environment Incus nerdctl client: %w", err)
	}
	if strings.TrimSpace(stat.Stdout) != "755:0:0" {
		return fmt.Errorf("Environment Incus nerdctl client ownership/mode mismatch")
	}
	return nil
}

func (r *Runtime) disableLegacyOCIUnits(ctx context.Context, ref string) error {
	for _, unit := range legacyOCIUnits {
		if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
			"--", "systemctl", "cat", unit); err != nil {
			continue
		}
		if _, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project,
			"--", "systemctl", "disable", "--now", unit); err != nil {
			return fmt.Errorf("disable legacy nested OCI unit %s: %w", unit, err)
		}
	}
	return nil
}

func workloadShimSource() (path, digest string, available bool, err error) {
	candidate := strings.TrimSpace(os.Getenv(workloadShimBinaryEnvironment))
	if candidate == "" {
		executable, execErr := os.Executable()
		if execErr != nil {
			return "", "", false, nil
		}
		if resolved, resolveErr := filepath.EvalSymlinks(executable); resolveErr == nil {
			executable = resolved
		}
		candidate = filepath.Join(filepath.Dir(executable), "haco-host")
		if _, statErr := os.Lstat(candidate); os.IsNotExist(statErr) {
			return "", "", false, nil
		} else if statErr != nil {
			return "", "", false, fmt.Errorf("inspect Environment nerdctl companion: %w", statErr)
		}
	}
	candidate = filepath.Clean(candidate)
	if !filepath.IsAbs(candidate) {
		return "", "", false, fmt.Errorf("Environment nerdctl companion must be an absolute path")
	}
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", "", false, fmt.Errorf("inspect Environment nerdctl companion: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 || info.Mode().Perm()&0o111 == 0 {
		return "", "", false, fmt.Errorf("unsafe Environment nerdctl companion %q", candidate)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (stat.Uid != 0 && int(stat.Uid) != os.Geteuid()) {
		return "", "", false, fmt.Errorf("Environment nerdctl companion must be owned by root or effective uid")
	}
	file, err := os.Open(candidate)
	if err != nil {
		return "", "", false, fmt.Errorf("open Environment nerdctl companion: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", "", false, fmt.Errorf("hash Environment nerdctl companion: %w", err)
	}
	return candidate, fmt.Sprintf("%x", hash.Sum(nil)), true, nil
}

func writeEnvironmentNerdctlLauncher(environment string) (string, func(), error) {
	if err := validateWorkloadToken("environment", environment); err != nil {
		return "", func() {}, err
	}
	file, err := os.CreateTemp("", "haco-nerdctl-launcher-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create Environment nerdctl launcher: %w", err)
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}
	if err := file.Chmod(0o700); err != nil {
		cleanup()
		return "", func() {}, err
	}
	content := "#!/bin/sh\n" +
		environmentNerdctlLauncherMark + "\n" +
		"export HACO_CONTROL_SOCKET=" + environmentWorkloadSocket + "\n" +
		"export HACO_NERDCTL_NAMESPACE=" + environment + "\n" +
		"exec " + environmentNerdctlBinaryPath + " \"$@\"\n"
	if _, err := file.WriteString(content); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("write Environment nerdctl launcher: %w", err)
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return file.Name(), cleanup, nil
}
