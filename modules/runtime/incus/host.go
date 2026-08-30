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
