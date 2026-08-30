package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const trustedHostName = "haco-host"
const trustedHostRoleKey = "user.hacocoon.role"
const trustedHostRoleValue = "trusted-host"
const trustedHostClientPath = "/usr/local/bin/haco-host"
const trustedHostControlDevice = "haco-control"
const trustedHostControlSocket = "/run/hacocoon/control.sock"

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
		return r.ensureTrustedHostRunning(ctx, state)
	}

	_, initErr := r.runner.Run(ctx, "incus", "init", r.image, trustedHostName,
		"--project", r.project,
		"--storage", rootPool,
		"--config", trustedHostRoleKey+"="+trustedHostRoleValue,
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
		return r.ensureTrustedHostRunning(ctx, state)
	}

	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return fmt.Errorf("verify newly created trusted host: %w", err)
	}
	return r.ensureTrustedHostRunning(ctx, "STOPPED")
}

// ProvisionTrustedHostClient installs the client-only haco-host binary and a
// narrow Hacocoon-owned Unix-socket proxy into the trusted Host. It deliberately
// does not expose the Incus control socket or any Physical Host directory.
//
// The hostControlSocket is the Physical Host controller socket. Inside the
// instance the client always sees the stable trustedHostControlSocket path.
func (r *Runtime) ProvisionTrustedHostClient(ctx context.Context, clientBinary, hostControlSocket string) error {
	clientBinary = strings.TrimSpace(clientBinary)
	hostControlSocket = strings.TrimSpace(hostControlSocket)
	if clientBinary == "" || !strings.HasPrefix(hostControlSocket, "/") {
		return core.ErrInvalidArgument
	}
	if err := r.EnsureTrustedHost(ctx); err != nil {
		return err
	}
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}

	if _, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "install", "-d", "-m", "0755", "/usr/local/bin", "/run/hacocoon"); err != nil {
		return fmt.Errorf("prepare trusted host client directories: %w", err)
	}
	if _, err := r.runner.Run(ctx, "incus", "file", "push", clientBinary,
		trustedHostName+trustedHostClientPath,
		"--project", r.project,
		"--uid", "0", "--gid", "0", "--mode", "0755"); err != nil {
		return fmt.Errorf("install haco-host client binary: %w", err)
	}
	if err := r.ensureTrustedHostControlProxy(ctx, hostControlSocket); err != nil {
		return err
	}

	result, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", trustedHostClientPath, "doctor")
	if err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason == "" {
			reason = strings.TrimSpace(result.Stdout)
		}
		if reason == "" {
			reason = err.Error()
		}
		return fmt.Errorf("verify trusted host client control path: %s: %w", reason, err)
	}
	return nil
}

func (r *Runtime) ensureTrustedHostControlProxy(ctx context.Context, hostControlSocket string) error {
	expected := map[string]string{
		"listen":  "unix:" + trustedHostControlSocket,
		"connect": "unix:" + hostControlSocket,
		"bind":    "instance",
		"uid":     "0",
		"gid":     "0",
		"mode":    "0600",
	}

	listenResult, inspectErr := r.runner.Run(ctx, "incus", "config", "device", "get",
		trustedHostName, trustedHostControlDevice, "listen", "--project", r.project)
	if inspectErr != nil {
		args := []string{
			"config", "device", "add", trustedHostName, trustedHostControlDevice, "proxy",
			"listen=" + expected["listen"],
			"connect=" + expected["connect"],
			"bind=" + expected["bind"],
			"uid=" + expected["uid"],
			"gid=" + expected["gid"],
			"mode=" + expected["mode"],
			"--project", r.project,
		}
		if _, err := r.runner.Run(ctx, "incus", args...); err != nil {
			return errors.Join(
				fmt.Errorf("inspect trusted host control device: %w", inspectErr),
				fmt.Errorf("add trusted host control proxy: %w", err),
			)
		}
		return nil
	}

	observed := map[string]string{"listen": strings.TrimSpace(listenResult.Stdout)}
	for _, key := range []string{"connect", "bind", "uid", "gid", "mode"} {
		result, err := r.runner.Run(ctx, "incus", "config", "device", "get",
			trustedHostName, trustedHostControlDevice, key, "--project", r.project)
		if err != nil {
			return fmt.Errorf("inspect trusted host control device %s: %w", key, err)
		}
		observed[key] = strings.TrimSpace(result.Stdout)
	}
	for key, want := range expected {
		got := observed[key]
		if key == "mode" {
			got = strings.TrimLeft(got, "0")
			want = strings.TrimLeft(want, "0")
			if got == "" {
				got = "0"
			}
			if want == "" {
				want = "0"
			}
		}
		if got != want {
			return fmt.Errorf("trusted host control device %s=%q, want %q; refusing implicit takeover: %w",
				key, observed[key], expected[key], core.ErrIncompatibleState)
		}
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
