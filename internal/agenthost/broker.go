package agenthost

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const maxSessionIDBytes = 256

type Spec struct {
	SessionID     string                   `json:"session_id"`
	WorkspacePath string                   `json:"workspace_path"`
	AccessMode    core.WorkspaceAccessMode `json:"access_mode"`
}

type Binding struct {
	SessionID       string                   `json:"session_id"`
	EnvironmentName string                   `json:"environment"`
	WorkspacePath   string                   `json:"workspace_path"`
	AccessMode      core.WorkspaceAccessMode `json:"access_mode"`
}

type environmentLifecycle interface {
	Create(context.Context, core.EnvironmentSpec) (core.Environment, error)
	Delete(context.Context, string) error
}

type environmentStore interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
}

// Broker binds an opaque external agent-session identity to one Hacocoon
// Environment. It deliberately lives outside Core: agent/session concepts are
// client-integration concerns, while the existing Workspace/Environment model
// remains the security and lifecycle boundary underneath it.
//
// The broker is control-plane code. It is not intended to be mounted into, or
// invoked from, the untrusted Environment that runs the agent.
type Broker struct {
	environments environmentLifecycle
	store        environmentStore
}

func New(environments environmentLifecycle, store environmentStore) *Broker {
	return &Broker{environments: environments, store: store}
}

func (b *Broker) Acquire(ctx context.Context, spec Spec) (Binding, error) {
	if b == nil || b.environments == nil || b.store == nil {
		return Binding{}, core.ErrInvalidArgument
	}
	if err := validateSessionID(spec.SessionID); err != nil {
		return Binding{}, err
	}
	workspace, err := canonicalWorkspace(spec.WorkspacePath)
	if err != nil {
		return Binding{}, err
	}
	mode, err := normalizeAccessMode(spec.AccessMode)
	if err != nil {
		return Binding{}, err
	}
	name := environmentName(spec.SessionID)

	existing, err := b.store.GetEnvironment(ctx, name)
	if err == nil {
		return bindingForExisting(spec.SessionID, workspace, mode, existing)
	}
	if !isNotFound(err) {
		return Binding{}, err
	}

	created, err := b.environments.Create(ctx, core.EnvironmentSpec{
		Name:          name,
		WorkspacePath: workspace,
		AccessMode:    mode,
	})
	if err != nil {
		// A concurrent control-plane process may have won the deterministic
		// name race. Re-read and accept only an exact matching binding.
		if errors.Is(err, core.ErrAlreadyExists) {
			existing, getErr := b.store.GetEnvironment(ctx, name)
			if getErr == nil {
				return bindingForExisting(spec.SessionID, workspace, mode, existing)
			}
		}
		return Binding{}, fmt.Errorf("acquire agent environment %q: %w", name, err)
	}
	return bindingFromEnvironment(spec.SessionID, created), nil
}

func (b *Broker) Lookup(ctx context.Context, sessionID string) (Binding, error) {
	if b == nil || b.store == nil {
		return Binding{}, core.ErrInvalidArgument
	}
	if err := validateSessionID(sessionID); err != nil {
		return Binding{}, err
	}
	environment, err := b.store.GetEnvironment(ctx, environmentName(sessionID))
	if err != nil {
		return Binding{}, err
	}
	return bindingFromEnvironment(sessionID, environment), nil
}

func (b *Broker) Release(ctx context.Context, sessionID string) error {
	if b == nil || b.environments == nil {
		return core.ErrInvalidArgument
	}
	if err := validateSessionID(sessionID); err != nil {
		return err
	}
	if err := b.environments.Delete(ctx, environmentName(sessionID)); err != nil {
		return fmt.Errorf("release agent environment: %w", err)
	}
	return nil
}

func bindingForExisting(sessionID, workspace string, mode core.WorkspaceAccessMode, environment core.Environment) (Binding, error) {
	if filepath.Clean(environment.Workspace.Path) != filepath.Clean(workspace) || environment.AccessMode != mode {
		return Binding{}, fmt.Errorf(
			"agent session %q is already bound to environment %q with workspace %q (%s): %w",
			sessionID,
			environment.Name,
			environment.Workspace.Path,
			environment.AccessMode,
			core.ErrAlreadyExists,
		)
	}
	return bindingFromEnvironment(sessionID, environment), nil
}

func bindingFromEnvironment(sessionID string, environment core.Environment) Binding {
	return Binding{
		SessionID:       sessionID,
		EnvironmentName: environment.Name,
		WorkspacePath:   environment.Workspace.Path,
		AccessMode:      environment.AccessMode,
	}
}

func environmentName(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return fmt.Sprintf("agent-%x", sum[:10])
}

func validateSessionID(sessionID string) error {
	if sessionID == "" || len(sessionID) > maxSessionIDBytes || !utf8.ValidString(sessionID) || strings.TrimSpace(sessionID) != sessionID {
		return fmt.Errorf("invalid agent session id: %w", core.ErrInvalidArgument)
	}
	for _, r := range sessionID {
		if unicode.IsControl(r) {
			return fmt.Errorf("invalid agent session id: %w", core.ErrInvalidArgument)
		}
	}
	return nil
}

func canonicalWorkspace(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("workspace path is required: %w", core.ErrInvalidArgument)
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve workspace %q: %w", path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("stat workspace %q: %w", resolved, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("workspace %q is not a directory: %w", resolved, core.ErrInvalidArgument)
	}
	return filepath.Clean(resolved), nil
}

func normalizeAccessMode(mode core.WorkspaceAccessMode) (core.WorkspaceAccessMode, error) {
	if mode == "" {
		return core.WorkspaceReadWrite, nil
	}
	switch mode {
	case core.WorkspaceReadOnly, core.WorkspaceReadWrite:
		return mode, nil
	default:
		return "", fmt.Errorf("workspace access mode %q: %w", mode, core.ErrInvalidArgument)
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, core.ErrNotFound) || os.IsNotExist(err)
}
