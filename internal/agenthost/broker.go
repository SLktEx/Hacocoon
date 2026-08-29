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
// Environment. Agent/session concepts stay outside Core, while the existing
// Workspace/Environment lifecycle remains the security boundary underneath.
//
// The broker is trusted control-plane code. It is not intended to be mounted
// into, or invoked from, the untrusted Environment that runs the coding agent.
type Broker struct {
	environments environmentLifecycle
	envStore     environmentStore
	bindings     bindingStore
}

func New(environments environmentLifecycle, envStore environmentStore, bindings bindingStore) *Broker {
	return &Broker{environments: environments, envStore: envStore, bindings: bindings}
}

func (b *Broker) Acquire(ctx context.Context, spec Spec) (Binding, error) {
	if b == nil || b.environments == nil || b.envStore == nil || b.bindings == nil {
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
	key := sessionKey(spec.SessionID)

	record, err := b.bindings.Get(ctx, key)
	if err == nil {
		return b.bindingFromRecord(ctx, spec.SessionID, workspace, mode, record)
	}
	if !isNotFound(err) {
		return Binding{}, err
	}

	name := environmentNameFromKey(key)
	created, err := b.environments.Create(ctx, core.EnvironmentSpec{
		Name:          name,
		WorkspacePath: workspace,
		AccessMode:    mode,
	})
	if err != nil {
		// Without a persisted binding, an existing deterministic Environment
		// has no provenance proving that it belongs to this session. Refuse
		// to adopt it, even if its Workspace happens to match.
		return Binding{}, fmt.Errorf("acquire agent environment %q: %w", name, err)
	}

	record = bindingRecord{
		EnvironmentName: created.Name,
		WorkspacePath:   created.Workspace.Path,
		AccessMode:      created.AccessMode,
	}
	if err := b.bindings.PutIfAbsent(ctx, key, record); err != nil {
		cleanupErr := b.environments.Delete(context.WithoutCancel(ctx), created.Name)
		return Binding{}, errors.Join(
			fmt.Errorf("persist agent binding: %w", err),
			wrapCleanupError(cleanupErr),
		)
	}
	return bindingFromRecord(spec.SessionID, record), nil
}

func (b *Broker) Lookup(ctx context.Context, sessionID string) (Binding, error) {
	if b == nil || b.envStore == nil || b.bindings == nil {
		return Binding{}, core.ErrInvalidArgument
	}
	if err := validateSessionID(sessionID); err != nil {
		return Binding{}, err
	}
	record, err := b.bindings.Get(ctx, sessionKey(sessionID))
	if err != nil {
		return Binding{}, err
	}
	environment, err := b.envStore.GetEnvironment(ctx, record.EnvironmentName)
	if err != nil {
		return Binding{}, fmt.Errorf("resolve bound environment %q: %w", record.EnvironmentName, err)
	}
	if err := verifyRecord(record, environment); err != nil {
		return Binding{}, err
	}
	return bindingFromRecord(sessionID, record), nil
}

func (b *Broker) Release(ctx context.Context, sessionID string) error {
	if b == nil || b.environments == nil || b.bindings == nil {
		return core.ErrInvalidArgument
	}
	if err := validateSessionID(sessionID); err != nil {
		return err
	}
	key := sessionKey(sessionID)
	record, err := b.bindings.Get(ctx, key)
	if err != nil {
		return err
	}
	if err := b.environments.Delete(ctx, record.EnvironmentName); err != nil {
		return fmt.Errorf("release agent environment %q: %w", record.EnvironmentName, err)
	}
	if err := b.bindings.Delete(context.WithoutCancel(ctx), key); err != nil {
		return fmt.Errorf("delete released agent binding: %w", err)
	}
	return nil
}

func (b *Broker) bindingFromRecord(ctx context.Context, sessionID, workspace string, mode core.WorkspaceAccessMode, record bindingRecord) (Binding, error) {
	if filepath.Clean(record.WorkspacePath) != filepath.Clean(workspace) || record.AccessMode != mode {
		return Binding{}, fmt.Errorf(
			"agent session is already bound to environment %q with workspace %q (%s): %w",
			record.EnvironmentName,
			record.WorkspacePath,
			record.AccessMode,
			core.ErrAlreadyExists,
		)
	}
	environment, err := b.envStore.GetEnvironment(ctx, record.EnvironmentName)
	if err != nil {
		return Binding{}, fmt.Errorf("resolve bound environment %q: %w", record.EnvironmentName, err)
	}
	if err := verifyRecord(record, environment); err != nil {
		return Binding{}, err
	}
	return bindingFromRecord(sessionID, record), nil
}

func verifyRecord(record bindingRecord, environment core.Environment) error {
	if environment.Name != record.EnvironmentName ||
		filepath.Clean(environment.Workspace.Path) != filepath.Clean(record.WorkspacePath) ||
		environment.AccessMode != record.AccessMode {
		return fmt.Errorf("agent binding for environment %q does not match persisted environment metadata: %w", record.EnvironmentName, core.ErrIncompatibleState)
	}
	return nil
}

func bindingFromRecord(sessionID string, record bindingRecord) Binding {
	return Binding{
		SessionID:       sessionID,
		EnvironmentName: record.EnvironmentName,
		WorkspacePath:   record.WorkspacePath,
		AccessMode:      record.AccessMode,
	}
}

func sessionKey(sessionID string) string {
	sum := sha256.Sum256([]byte(sessionID))
	return fmt.Sprintf("%x", sum[:16])
}

func environmentName(sessionID string) string {
	return environmentNameFromKey(sessionKey(sessionID))
}

func environmentNameFromKey(key string) string {
	return "agent-" + key[:20]
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

func wrapCleanupError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("cleanup unbound agent environment: %w", err)
}

func isNotFound(err error) bool {
	return errors.Is(err, core.ErrNotFound) || os.IsNotExist(err)
}
