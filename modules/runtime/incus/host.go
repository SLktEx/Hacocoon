package incus

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	trustedHostInstance = "haco-host"
	trustedHostRoleKey  = "user.hacocoon.role"
	trustedHostRole     = "trusted-host"
)

// EnsureTrustedHost reconciles the persistent trusted Hacocoon management
// instance. The instance is infrastructure, not an untrusted Environment.
func (r *Runtime) EnsureTrustedHost(ctx context.Context) error {
	if err := r.ensureProject(ctx); err != nil {
		return fmt.Errorf("ensure Incus project for trusted host: %w", err)
	}
	rootPool, err := r.defaultRootPool(ctx)
	if err != nil {
		return fmt.Errorf("resolve trusted host root storage: %w", err)
	}

	exists, state, err := r.trustedHostState(ctx)
	if err != nil {
		return err
	}
	if exists {
		return r.reconcileTrustedHostState(ctx, state)
	}

	_, createErr := r.runner.Run(ctx, "incus", "init", r.image, trustedHostInstance,
		"--project", r.project,
		"--storage", rootPool,
		"--config", trustedHostRoleKey+"="+trustedHostRole,
	)
	if createErr != nil {
		// Concurrent/bootstrap retry: never adopt by name alone. Re-inspect and
		// accept only the exact Hacocoon infrastructure marker.
		exists, state, inspectErr := r.trustedHostState(ctx)
		if inspectErr != nil {
			return errors.Join(fmt.Errorf("create trusted host: %w", createErr), inspectErr)
		}
		if !exists {
			return fmt.Errorf("create trusted host: %w", createErr)
		}
		if err := r.reconcileTrustedHostState(ctx, state); err != nil {
			return errors.Join(fmt.Errorf("create trusted host: %w", createErr), err)
		}
		return nil
	}

	if _, err := r.runner.Run(ctx, "incus", "start", trustedHostInstance, "--project", r.project); err != nil {
		cleanupErr := r.deleteTrustedHostIfOwned(ctx)
		return errors.Join(fmt.Errorf("start trusted host: %w", err), cleanupErr)
	}
	return nil
}

// ShellTrustedHost opens an interactive login shell in the trusted management
// instance. EnsureTrustedHost must have succeeded before this method is used.
func (r *Runtime) ShellTrustedHost(ctx context.Context) error {
	exists, state, err := r.trustedHostState(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("trusted host %q is missing: %w", trustedHostInstance, core.ErrNotFound)
	}
	if err := r.reconcileTrustedHostState(ctx, state); err != nil {
		return err
	}
	_, err = r.execInteractive(ctx, trustedHostInstance, []string{"/bin/bash", "-l"})
	return err
}

type trustedHostObserved struct {
	Status string `json:"status"`
	Config map[string]string `json:"config"`
}

func (r *Runtime) trustedHostState(ctx context.Context) (bool, trustedHostObserved, error) {
	result, err := r.runner.Run(ctx, "incus", "list", trustedHostInstance,
		"--project", r.project, "--format", "json")
	if err != nil {
		return false, trustedHostObserved{}, fmt.Errorf("inspect trusted host: %w", err)
	}
	trimmed := strings.TrimSpace(result.Stdout)
	if trimmed == "" || trimmed == "[]" {
		return false, trustedHostObserved{}, nil
	}
	var rows []trustedHostObserved
	if err := jsonUnmarshalStrict([]byte(trimmed), &rows); err != nil {
		return false, trustedHostObserved{}, fmt.Errorf("decode trusted host state: %w", err)
	}
	if len(rows) == 0 {
		return false, trustedHostObserved{}, nil
	}
	if len(rows) != 1 {
		return false, trustedHostObserved{}, fmt.Errorf("trusted host query returned %d instances: %w", len(rows), core.ErrIncompatibleState)
	}
	if rows[0].Config[trustedHostRoleKey] != trustedHostRole {
		return false, trustedHostObserved{}, fmt.Errorf("Incus instance %q exists without exact Hacocoon trusted-host marker; refusing adoption: %w", trustedHostInstance, core.ErrIncompatibleState)
	}
	return true, rows[0], nil
}

func (r *Runtime) reconcileTrustedHostState(ctx context.Context, state trustedHostObserved) error {
	switch strings.ToUpper(strings.TrimSpace(state.Status)) {
	case "RUNNING":
		return nil
	case "STOPPED":
		if _, err := r.runner.Run(ctx, "incus", "start", trustedHostInstance, "--project", r.project); err != nil {
			return fmt.Errorf("start trusted host: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("trusted host is in unexpected state %q: %w", state.Status, core.ErrIncompatibleState)
	}
}

func (r *Runtime) deleteTrustedHostIfOwned(ctx context.Context) error {
	exists, _, err := r.trustedHostState(ctx)
	if err != nil || !exists {
		return err
	}
	_, err = r.runner.Run(ctx, "incus", "delete", trustedHostInstance, "--project", r.project, "--force")
	if err != nil {
		return fmt.Errorf("cleanup trusted host: %w", err)
	}
	return nil
}

func jsonUnmarshalStrict(data []byte, dst any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}
