package incus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const trustedHostName = "haco-host"
const trustedHostRoleKey = "user.hacocoon.role"
const trustedHostRoleValue = "trusted-host"

const trustedHostControlDevice = "haco-control"
const trustedHostControlSocket = "/var/lib/hacocoon-control.sock"
const trustedHostControlEnvKey = "environment.HACO_CONTROL_SOCKET"
const defaultPhysicalHostControlSocket = "/run/hacocoon/control.sock"
const trustedHostClientPath = "/usr/local/bin/haco-host"

type trustedHostListEntry struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// EnsureTrustedHost reconciles the persistent Hacocoon-owned trusted Host
// instance. The instance is intentionally not a normal Environment: it has no
// Workspace lease and does not receive the managed sandbox profile.
func (r *Runtime) EnsureTrustedHost(ctx context.Context) error {
	if err := r.ensureProject(ctx); err != nil {
		return fmt.Errorf("ensure Incus project for trusted host: %w", err)
	}
	rootPool, err := r.defaultRootPool(ctx)
	if err != nil {
		return fmt.Errorf("resolve trusted host root storage: %w", err)
	}

	state, exists, err := r.trustedHostState(ctx)
	if err != nil {
		return err
	}
	if exists {
		if err := r.verifyTrustedHostOwnership(ctx); err != nil {
			return err
		}
		if err := r.ensureTrustedHostClientEnvironment(ctx); err != nil {
			return err
		}
		if err := r.ensureTrustedHostControlDevice(ctx); err != nil {
			return err
		}
		return r.ensureTrustedHostNetworkAndRunning(ctx, state, rootPool)
	}

	if err := r.ensureTrustedHostNetwork(ctx); err != nil {
		return err
	}
	_, initErr := r.runner.Run(ctx, "incus", "init", r.image, trustedHostName,
		"--project", r.project,
		"--storage", rootPool,
		"--no-profiles", "--network", trustedHostNetwork,
		"--config", trustedHostRoleKey+"="+trustedHostRoleValue,
		"--config", trustedHostControlEnvKey+"="+trustedHostControlSocket,
	)
	if initErr != nil {
		// Another reconciler may have won the create race. Only adopt the result
		// when exact Hacocoon ownership can be proven from the marker.
		state, exists, inspectErr := r.trustedHostState(ctx)
		if inspectErr != nil || !exists {
			return errors.Join(fmt.Errorf("create trusted host: %w", initErr), inspectErr)
		}
		if err := r.verifyTrustedHostOwnership(ctx); err != nil {
			return errors.Join(fmt.Errorf("create trusted host: %w", initErr), err)
		}
		if err := r.ensureTrustedHostClientEnvironment(ctx); err != nil {
			return errors.Join(fmt.Errorf("create trusted host: %w", initErr), err)
		}
		if err := r.ensureTrustedHostControlDevice(ctx); err != nil {
			return errors.Join(fmt.Errorf("create trusted host: %w", initErr), err)
		}
		return r.ensureTrustedHostNetworkAndRunning(ctx, state, rootPool)
	}

	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return fmt.Errorf("verify newly created trusted host: %w", err)
	}
	if err := r.ensureTrustedHostClientEnvironment(ctx); err != nil {
		return err
	}
	if err := r.ensureTrustedHostControlDevice(ctx); err != nil {
		return err
	}
	return r.ensureTrustedHostNetworkAndRunning(ctx, "STOPPED", rootPool)
}

// ProvisionTrustedHostClient installs the client-only haco-host binary into the
// already-reconciled trusted instance. The source must be an executable owned
// by the caller and must not be writable by another local user.
func (r *Runtime) ProvisionTrustedHostClient(ctx context.Context, source string) error {
	state, exists, err := r.trustedHostState(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("trusted host is missing: %w", core.ErrNotFound)
	}
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}
	if state != "RUNNING" {
		return fmt.Errorf("trusted host must be running before client provisioning, got %q: %w", state, core.ErrIncompatibleState)
	}

	source, digest, err := trustedClientSource(source)
	if err != nil {
		return err
	}
	if ok, _ := r.trustedHostClientMatches(ctx, digest); ok {
		return nil
	}

	if _, err := r.runner.Run(ctx, "incus", "file", "push", source,
		trustedHostName+trustedHostClientPath,
		"--project", r.project,
		"--create-dirs",
		"--uid", "0",
		"--gid", "0",
		"--mode", "0755",
	); err != nil {
		return fmt.Errorf("install trusted host client: %w", err)
	}
	ok, verifyErr := r.trustedHostClientMatches(ctx, digest)
	if verifyErr != nil {
		return fmt.Errorf("verify trusted host client: %w", verifyErr)
	}
	if !ok {
		return fmt.Errorf("trusted host client verification mismatch: %w", core.ErrIncompatibleState)
	}
	return nil
}

// ShellTrustedHost ensures the trusted host exists and then opens an
// interactive login shell. Incus control authority stays on the Physical Host;
// no Incus socket is mounted into haco-host.
func (r *Runtime) ShellTrustedHost(ctx context.Context) error {
	if err := r.EnsureTrustedHost(ctx); err != nil {
		return err
	}
	_, err := r.execInteractive(ctx, trustedHostName, []string{"/bin/bash", "-l"})
	return err
}

func (r *Runtime) ensureTrustedHostClientEnvironment(ctx context.Context) error {
	result, err := r.runner.Run(ctx, "incus", "config", "get", trustedHostName, trustedHostControlEnvKey, "--project", r.project)
	if err != nil {
		return fmt.Errorf("read trusted host control environment: %w", err)
	}
	current := strings.TrimSpace(result.Stdout)
	if current == trustedHostControlSocket {
		return nil
	}
	if current != "" {
		return fmt.Errorf("trusted host control environment mismatch: got %q want %q: %w", current, trustedHostControlSocket, core.ErrIncompatibleState)
	}
	if _, err := r.runner.Run(ctx, "incus", "config", "set", trustedHostName,
		trustedHostControlEnvKey+"="+trustedHostControlSocket, "--project", r.project); err != nil {
		return fmt.Errorf("set trusted host control environment: %w", err)
	}
	result, err = r.runner.Run(ctx, "incus", "config", "get", trustedHostName, trustedHostControlEnvKey, "--project", r.project)
	if err != nil {
		return fmt.Errorf("verify trusted host control environment: %w", err)
	}
	if strings.TrimSpace(result.Stdout) != trustedHostControlSocket {
		return fmt.Errorf("trusted host control environment did not converge: %w", core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) ensureTrustedHostControlDevice(ctx context.Context) error {
	hostSocket, err := physicalHostControlSocket()
	if err != nil {
		return err
	}
	result, err := r.runner.Run(ctx, "incus", "config", "device", "list", trustedHostName, "--project", r.project)
	if err != nil {
		return fmt.Errorf("list trusted host devices: %w", err)
	}
	found := false
	for _, name := range strings.Fields(result.Stdout) {
		if name == trustedHostControlDevice {
			found = true
			break
		}
	}
	if found {
		return r.verifyTrustedHostControlDevice(ctx, hostSocket)
	}

	_, addErr := r.runner.Run(ctx, "incus", "config", "device", "add",
		trustedHostName, trustedHostControlDevice, "proxy",
		"bind=instance",
		"listen=unix:"+trustedHostControlSocket,
		"connect=unix:"+hostSocket,
		"mode=0600",
		"uid=0",
		"gid=0",
		"--project", r.project,
	)
	if addErr != nil {
		// A concurrent reconciler may have added the device. Accept that race
		// only if the final device is exactly the narrow Hacocoon proxy.
		if verifyErr := r.verifyTrustedHostControlDevice(ctx, hostSocket); verifyErr == nil {
			return nil
		} else {
			return errors.Join(fmt.Errorf("add trusted host control proxy: %w", addErr), verifyErr)
		}
	}
	return r.verifyTrustedHostControlDevice(ctx, hostSocket)
}

func (r *Runtime) verifyTrustedHostControlDevice(ctx context.Context, hostSocket string) error {
	expected := []struct {
		key   string
		value string
	}{
		{"type", "proxy"},
		{"bind", "instance"},
		{"listen", "unix:" + trustedHostControlSocket},
		{"connect", "unix:" + hostSocket},
		{"mode", "0600"},
		{"uid", "0"},
		{"gid", "0"},
		{"nat", ""},
		{"proxy_protocol", ""},
		{"security.uid", ""},
		{"security.gid", ""},
	}
	for _, item := range expected {
		result, err := r.runner.Run(ctx, "incus", "config", "device", "get",
			trustedHostName, trustedHostControlDevice, item.key, "--project", r.project)
		if err != nil {
			return fmt.Errorf("inspect trusted host control proxy %s: %w", item.key, err)
		}
		if strings.TrimSpace(result.Stdout) != item.value {
			return fmt.Errorf("trusted host control proxy %s mismatch: got %q want %q: %w",
				item.key, strings.TrimSpace(result.Stdout), item.value, core.ErrIncompatibleState)
		}
	}
	return nil
}

func physicalHostControlSocket() (string, error) {
	path := defaultPhysicalHostControlSocket
	// A non-root process may override the endpoint for tests/development. The
	// root-authority path deliberately ignores inherited environment overrides
	// so narrow sudo entry cannot be redirected to an arbitrary Host socket.
	if os.Geteuid() != 0 {
		if configured := strings.TrimSpace(os.Getenv("HACO_CONTROL_SOCKET")); configured != "" {
			path = configured
		}
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsRune(path, '\x00') {
		return "", fmt.Errorf("invalid Physical Host control socket %q: %w", path, core.ErrInvalidArgument)
	}
	return path, nil
}

func trustedClientSource(source string) (string, string, error) {
	source = strings.TrimSpace(source)
	if source == "" || !filepath.IsAbs(source) || filepath.Clean(source) != source {
		return "", "", fmt.Errorf("invalid trusted host client source %q: %w", source, core.ErrInvalidArgument)
	}
	info, err := os.Lstat(source)
	if err != nil {
		return "", "", fmt.Errorf("inspect trusted host client source: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o022 != 0 || info.Mode().Perm()&0o111 == 0 {
		return "", "", fmt.Errorf("unsafe trusted host client source %q: %w", source, core.ErrIncompatibleState)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Geteuid() {
		return "", "", fmt.Errorf("trusted host client source is not owned by effective uid %d: %w", os.Geteuid(), core.ErrIncompatibleState)
	}
	file, err := os.Open(source)
	if err != nil {
		return "", "", fmt.Errorf("open trusted host client source: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", "", fmt.Errorf("hash trusted host client source: %w", err)
	}
	return source, hex.EncodeToString(hash.Sum(nil)), nil
}

func (r *Runtime) trustedHostClientMatches(ctx context.Context, digest string) (bool, error) {
	hashResult, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "sha256sum", trustedHostClientPath)
	if err != nil {
		return false, err
	}
	fields := strings.Fields(hashResult.Stdout)
	if len(fields) < 1 || fields[0] != digest {
		return false, nil
	}
	statResult, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "stat", "-c", "%a:%u:%g", trustedHostClientPath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(statResult.Stdout) == "755:0:0", nil
}

func (r *Runtime) trustedHostState(ctx context.Context) (string, bool, error) {
	result, err := r.runner.Run(ctx, "incus", "list", trustedHostName,
		"--project", r.project,
		"--format", "json",
	)
	if err != nil {
		return "", false, fmt.Errorf("inspect trusted host: %w", err)
	}
	var entries []trustedHostListEntry
	if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
		return "", false, fmt.Errorf("decode trusted host inventory: %w", err)
	}
	var exact *trustedHostListEntry
	for i := range entries {
		if entries[i].Name != trustedHostName {
			continue
		}
		if exact != nil {
			return "", false, fmt.Errorf("duplicate exact trusted host inventory entries: %w", core.ErrIncompatibleState)
		}
		exact = &entries[i]
	}
	if exact == nil {
		return "", false, nil
	}
	return strings.ToUpper(strings.TrimSpace(exact.Status)), true, nil
}

func (r *Runtime) verifyTrustedHostOwnership(ctx context.Context) error {
	result, err := r.runner.Run(ctx, "incus", "config", "get", trustedHostName, trustedHostRoleKey, "--project", r.project)
	if err != nil {
		return fmt.Errorf("read trusted host ownership marker: %w", err)
	}
	if strings.TrimSpace(result.Stdout) != trustedHostRoleValue {
		return fmt.Errorf("Incus instance %q is not owned as the Hacocoon trusted host; refusing takeover: %w", trustedHostName, core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) ensureTrustedHostRunning(ctx context.Context, state string) error {
	switch strings.ToUpper(strings.TrimSpace(state)) {
	case "RUNNING":
		return nil
	case "STOPPED":
		if _, err := r.runner.Run(ctx, "incus", "start", trustedHostName, "--project", r.project); err != nil {
			// Treat a concurrent successful start as success, but do not hide any
			// other unexpected state.
			observed, exists, inspectErr := r.trustedHostState(ctx)
			if inspectErr == nil && exists && observed == "RUNNING" {
				return nil
			}
			return errors.Join(fmt.Errorf("start trusted host: %w", err), inspectErr)
		}
		return nil
	default:
		return fmt.Errorf("trusted host is in unsupported Incus state %q: %w", state, core.ErrIncompatibleState)
	}
}
