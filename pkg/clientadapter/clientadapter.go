package clientadapter

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

const WorkspacePath = "/workspace"

type AccessMode string

const (
	ReadWrite AccessMode = "read-write"
	ReadOnly  AccessMode = "read-only"
)

type State string

const (
	Running State = "running"
	Stopped State = "stopped"
	Unknown State = "unknown"
)

type Environment struct {
	Name            string     `json:"name"`
	SourceWorkspace string     `json:"source_workspace"`
	WorkspacePath   string     `json:"workspace_path"`
	AccessMode      AccessMode `json:"access_mode"`
	State           State      `json:"state"`
}

type Connection struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	User       string `json:"user,omitempty"`
}

type EnsureRequest struct {
	Name          string
	WorkspacePath string
	AccessMode    AccessMode
}

type SSHRequest struct {
	Environment string
	PublicKey   string
	HostPort    int
}

type ForwardRequest struct {
	Environment string
	HostPort    int
	TargetPort  int
}

var (
	ErrInvalidArgument   = errors.New("invalid client adapter argument")
	ErrNotFound          = errors.New("client adapter target not found")
	ErrAlreadyExists     = errors.New("client adapter target already exists")
	ErrUnsupported       = errors.New("client adapter operation unsupported")
	ErrIncompatibleState = errors.New("client adapter incompatible state")
	ErrRecoveryRequired  = errors.New("client adapter recovery required")
	ErrUnavailable       = errors.New("client adapter unavailable")
	ErrBusy              = errors.New("client adapter target busy")
)

type environmentService interface {
	Create(context.Context, core.EnvironmentSpec) (core.Environment, error)
	Delete(context.Context, string) error
}

type clientService interface {
	Status(context.Context, string) (core.EnvironmentStatus, error)
	Connections(context.Context, string) ([]core.ClientConnection, error)
	Forward(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error)
	Unforward(context.Context, string, string) error
	SSH(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error)
}

type eventReader interface {
	Batch(context.Context, int64, int) (interaction.Batch, error)
}

type Adapter struct {
	environments environmentService
	clients      clientService
	events       eventReader
}

// NewLocal opens the client-neutral adapter against the local Hacocoon Host.
// It does not create an Environment and does not read or own client private keys.
func NewLocal(ctx context.Context) (*Adapter, error) {
	app, err := composition.Local(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	reader, err := interaction.NewDefaultReader()
	if err != nil {
		return nil, err
	}
	return newAdapter(app.Environments, app.Clients, reader), nil
}

func newAdapter(environments environmentService, clients clientService, events eventReader) *Adapter {
	return &Adapter{environments: environments, clients: clients, events: events}
}

// Ensure selects an existing Environment when its Workspace and access mode
// exactly match the request, otherwise it creates a new one. It never silently
// reuses an Environment with broader or different Workspace authority.
func (a *Adapter) Ensure(ctx context.Context, req EnsureRequest) (Environment, bool, error) {
	if a == nil || a.environments == nil || a.clients == nil || strings.TrimSpace(req.Name) == "" {
		return Environment{}, false, ErrInvalidArgument
	}
	workspace, err := canonicalWorkspace(req.WorkspacePath)
	if err != nil {
		return Environment{}, false, err
	}
	mode, coreMode, err := normalizeAccessMode(req.AccessMode)
	if err != nil {
		return Environment{}, false, err
	}

	status, err := a.clients.Status(ctx, req.Name)
	if err == nil {
		projected, projectErr := projectEnvironment(status)
		if projectErr != nil {
			return Environment{}, false, projectErr
		}
		if filepath.Clean(projected.SourceWorkspace) != workspace || projected.AccessMode != mode {
			return Environment{}, false, fmt.Errorf("environment %q already exists with different Workspace or access mode: %w", req.Name, ErrAlreadyExists)
		}
		return projected, false, nil
	}
	if !errors.Is(err, core.ErrNotFound) && !os.IsNotExist(err) {
		return Environment{}, false, translateError(err)
	}

	if _, err := a.environments.Create(ctx, core.EnvironmentSpec{
		Name:          req.Name,
		WorkspacePath: workspace,
		AccessMode:    coreMode,
	}); err != nil {
		return Environment{}, false, translateError(err)
	}
	status, err = a.clients.Status(ctx, req.Name)
	if err == nil {
		projected, projectErr := projectEnvironment(status)
		if projectErr == nil {
			return projected, true, nil
		}
		err = projectErr
	}
	cleanupErr := a.environments.Delete(context.WithoutCancel(ctx), req.Name)
	if cleanupErr != nil {
		return Environment{}, false, errors.Join(
			translateError(err),
			fmt.Errorf("cleanup newly-created environment %q: %v: %w", req.Name, cleanupErr, ErrRecoveryRequired),
		)
	}
	return Environment{}, false, translateError(err)
}

func (a *Adapter) Status(ctx context.Context, name string) (Environment, error) {
	if a == nil || a.clients == nil || strings.TrimSpace(name) == "" {
		return Environment{}, ErrInvalidArgument
	}
	status, err := a.clients.Status(ctx, name)
	if err != nil {
		return Environment{}, translateError(err)
	}
	return projectEnvironment(status)
}

func (a *Adapter) Connections(ctx context.Context, environment string) ([]Connection, error) {
	if a == nil || a.clients == nil || strings.TrimSpace(environment) == "" {
		return nil, ErrInvalidArgument
	}
	raw, err := a.clients.Connections(ctx, environment)
	if err != nil {
		return nil, translateError(err)
	}
	result := make([]Connection, 0, len(raw))
	for _, connection := range raw {
		projected, err := projectConnection(connection)
		if err != nil {
			return nil, err
		}
		result = append(result, projected)
	}
	return result, nil
}

// PrepareSSH installs only the supplied public key. The client retains and uses
// the corresponding private key itself.
func (a *Adapter) PrepareSSH(ctx context.Context, req SSHRequest) (Connection, error) {
	if a == nil || a.clients == nil || strings.TrimSpace(req.Environment) == "" || strings.TrimSpace(req.PublicKey) == "" {
		return Connection{}, ErrInvalidArgument
	}
	port, err := resolveHostPort(req.HostPort)
	if err != nil {
		return Connection{}, err
	}
	raw, err := a.clients.SSH(ctx, req.Environment, core.SSHAccessRequest{PublicKey: req.PublicKey, HostPort: port})
	if err != nil {
		return Connection{}, translateError(err)
	}
	projected, projectErr := projectConnection(raw)
	if projectErr == nil && projected.Kind == "ssh" && projected.TargetPort == 22 {
		return projected, nil
	}
	if projectErr == nil {
		projectErr = fmt.Errorf("provider returned non-SSH connection for SSH preparation: %w", ErrIncompatibleState)
	}
	return Connection{}, a.cleanupInvalidConnection(ctx, req.Environment, raw.ID, projectErr)
}

func (a *Adapter) Forward(ctx context.Context, req ForwardRequest) (Connection, error) {
	if a == nil || a.clients == nil || strings.TrimSpace(req.Environment) == "" || req.TargetPort < 1 || req.TargetPort > 65535 {
		return Connection{}, ErrInvalidArgument
	}
	port, err := resolveHostPort(req.HostPort)
	if err != nil {
		return Connection{}, err
	}
	raw, err := a.clients.Forward(ctx, req.Environment, core.LocalPortRequest{Protocol: "tcp", HostPort: port, TargetPort: req.TargetPort})
	if err != nil {
		return Connection{}, translateError(err)
	}
	projected, projectErr := projectConnection(raw)
	if projectErr == nil && projected.Kind == "tcp" && projected.TargetPort == req.TargetPort {
		return projected, nil
	}
	if projectErr == nil {
		projectErr = fmt.Errorf("provider returned unexpected forwarding metadata: %w", ErrIncompatibleState)
	}
	return Connection{}, a.cleanupInvalidConnection(ctx, req.Environment, raw.ID, projectErr)
}

func (a *Adapter) Revoke(ctx context.Context, environment, connectionID string) error {
	if a == nil || a.clients == nil || strings.TrimSpace(environment) == "" || strings.TrimSpace(connectionID) == "" {
		return ErrInvalidArgument
	}
	return translateError(a.clients.Unforward(ctx, environment, connectionID))
}

func (a *Adapter) Delete(ctx context.Context, environment string) error {
	if a == nil || a.environments == nil || strings.TrimSpace(environment) == "" {
		return ErrInvalidArgument
	}
	return translateError(a.environments.Delete(ctx, environment))
}

func (a *Adapter) InteractionBatch(ctx context.Context, offset int64, limit int) (interaction.Batch, error) {
	if a == nil || a.events == nil || offset < 0 || limit < 0 {
		return interaction.Batch{}, ErrInvalidArgument
	}
	if limit == 0 {
		limit = interaction.DefaultBatchSize
	}
	return a.events.Batch(ctx, offset, limit)
}

func (a *Adapter) cleanupInvalidConnection(ctx context.Context, environment, connectionID string, cause error) error {
	if strings.TrimSpace(connectionID) == "" {
		return errors.Join(cause, ErrRecoveryRequired)
	}
	if err := a.clients.Unforward(context.WithoutCancel(ctx), environment, connectionID); err != nil {
		return errors.Join(cause, fmt.Errorf("revoke incompatible connection %q: %v: %w", connectionID, err, ErrRecoveryRequired))
	}
	return cause
}

func canonicalWorkspace(path string) (string, error) {
	if path == "" {
		return "", ErrInvalidArgument
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve Workspace path: %v: %w", err, ErrInvalidArgument)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve Workspace path: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Workspace %q is not a directory: %w", resolved, ErrInvalidArgument)
	}
	return filepath.Clean(resolved), nil
}

func normalizeAccessMode(mode AccessMode) (AccessMode, core.WorkspaceAccessMode, error) {
	if mode == "" {
		mode = ReadWrite
	}
	switch mode {
	case ReadWrite:
		return mode, core.WorkspaceReadWrite, nil
	case ReadOnly:
		return mode, core.WorkspaceReadOnly, nil
	default:
		return "", "", ErrInvalidArgument
	}
}

func projectEnvironment(status core.EnvironmentStatus) (Environment, error) {
	mode := AccessMode("")
	switch status.Environment.AccessMode {
	case core.WorkspaceReadWrite:
		mode = ReadWrite
	case core.WorkspaceReadOnly:
		mode = ReadOnly
	default:
		return Environment{}, fmt.Errorf("unknown Workspace access mode %q: %w", status.Environment.AccessMode, ErrIncompatibleState)
	}
	state := State(status.State)
	switch state {
	case Running, Stopped, Unknown:
	default:
		return Environment{}, fmt.Errorf("unknown Environment state %q: %w", status.State, ErrIncompatibleState)
	}
	if status.Environment.Name == "" || status.Environment.Workspace.Path == "" {
		return Environment{}, ErrIncompatibleState
	}
	return Environment{
		Name:            status.Environment.Name,
		SourceWorkspace: filepath.Clean(status.Environment.Workspace.Path),
		WorkspacePath:   WorkspacePath,
		AccessMode:      mode,
		State:           state,
	}, nil
}

func projectConnection(raw core.ClientConnection) (Connection, error) {
	if strings.TrimSpace(raw.ID) == "" || raw.Port < 1 || raw.Port > 65535 || raw.TargetPort < 1 || raw.TargetPort > 65535 {
		return Connection{}, ErrIncompatibleState
	}
	ip := net.ParseIP(raw.Host)
	if ip == nil || !ip.IsLoopback() {
		return Connection{}, fmt.Errorf("connection %q is not loopback-only: %w", raw.ID, ErrIncompatibleState)
	}
	if raw.Kind != "tcp" && raw.Kind != "ssh" {
		return Connection{}, fmt.Errorf("connection %q has unsupported kind %q: %w", raw.ID, raw.Kind, ErrIncompatibleState)
	}
	return Connection{
		ID:         raw.ID,
		Kind:       raw.Kind,
		Host:       raw.Host,
		Port:       raw.Port,
		TargetPort: raw.TargetPort,
		User:       raw.User,
	}, nil
}

func resolveHostPort(port int) (int, error) {
	if port < 0 || port > 65535 {
		return 0, ErrInvalidArgument
	}
	if port != 0 {
		return port, nil
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("choose loopback port: %v: %w", err, ErrUnavailable)
	}
	defer listener.Close()
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.Port < 1 || address.Port > 65535 {
		return 0, ErrUnavailable
	}
	return address.Port, nil
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var sentinel error
	switch {
	case errors.Is(err, core.ErrRecoveryRequired):
		sentinel = ErrRecoveryRequired
	case errors.Is(err, core.ErrIncompatibleState):
		sentinel = ErrIncompatibleState
	case errors.Is(err, core.ErrInvalidArgument):
		sentinel = ErrInvalidArgument
	case errors.Is(err, core.ErrNotFound), os.IsNotExist(err):
		sentinel = ErrNotFound
	case errors.Is(err, core.ErrAlreadyExists):
		sentinel = ErrAlreadyExists
	case errors.Is(err, core.ErrUnsupported):
		sentinel = ErrUnsupported
	case errors.Is(err, core.ErrWorkspaceBusy), errors.Is(err, core.ErrStorageBusy):
		sentinel = ErrBusy
	case errors.Is(err, core.ErrRuntimeUnavailable), errors.Is(err, core.ErrStorageUnavailable):
		sentinel = ErrUnavailable
	default:
		return err
	}
	return fmt.Errorf("%v: %w", err, sentinel)
}
