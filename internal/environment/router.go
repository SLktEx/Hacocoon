package environment

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	ProviderIncus = "runtime.incus"
	ProviderEC2   = "runtime.ec2"
	refPrefix     = "haco-runtime-v1:"
)

// Provider is the v0.7 EnvironmentProvider seam. Provider-specific configuration
// and authority stay in the adapter. The stable Workspace/Environment lifecycle
// continues to depend only on the pre-existing Environment runtime contract.
type Provider interface {
	CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error)
	ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
	ShellEnvironment(context.Context, string) error
	DeleteEnvironment(context.Context, string) error
}

type InspectorProvider interface {
	InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error)
}

type LocalPortProvider interface {
	ForwardLocalPort(context.Context, string, core.LocalPortRequest) (core.ClientConnection, error)
	RemoveClientConnection(context.Context, string, string) error
}

type ConnectionListProvider interface {
	ListClientConnections(context.Context, string) ([]core.ClientConnection, error)
}

type SSHProvider interface {
	PrepareSSH(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error)
}

type SSHAccessProvider interface {
	PrepareSSHAccess(context.Context, string, core.SSHAccessRequest) (core.ClientConnection, error)
	RevokeSSHAccess(context.Context, string, string) error
}

type Registration struct {
	ID       string
	Provider Provider
}

func Register(id string, provider Provider) Registration {
	return Registration{ID: id, Provider: provider}
}

type Router struct {
	defaultProvider string
	providers       map[string]Provider
}

func NewRouter(defaultProvider string, registrations ...Registration) (*Router, error) {
	defaultProvider = strings.TrimSpace(defaultProvider)
	if defaultProvider == "" {
		return nil, core.ErrInvalidArgument
	}
	registered := make(map[string]Provider, len(registrations))
	for _, registration := range registrations {
		id := strings.TrimSpace(registration.ID)
		if id == "" || id != registration.ID || strings.ContainsAny(id, "\r\n\x00") || registration.Provider == nil {
			return nil, fmt.Errorf("invalid environment provider %q: %w", registration.ID, core.ErrInvalidArgument)
		}
		if _, exists := registered[id]; exists {
			return nil, fmt.Errorf("duplicate environment provider %q: %w", id, core.ErrAlreadyExists)
		}
		registered[id] = registration.Provider
	}
	if _, ok := registered[defaultProvider]; !ok {
		return nil, fmt.Errorf("default environment provider %q: %w", defaultProvider, core.ErrNotFound)
	}
	return &Router{defaultProvider: defaultProvider, providers: registered}, nil
}

func (r *Router) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	provider, err := r.provider(r.defaultProvider)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	created, err := provider.CreateEnvironment(ctx, spec)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if strings.TrimSpace(created.Ref) == "" {
		return core.EnvironmentRuntime{}, fmt.Errorf("provider %q returned empty runtime ref: %w", r.defaultProvider, core.ErrIncompatibleState)
	}
	return core.EnvironmentRuntime{Ref: encodeRouteRef(r.defaultProvider, created.Ref)}, nil
}

func (r *Router) ExecEnvironment(ctx context.Context, rawRef string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	provider, ref, err := r.resolve(rawRef)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	return provider.ExecEnvironment(ctx, ref, req)
}

func (r *Router) ShellEnvironment(ctx context.Context, rawRef string) error {
	provider, ref, err := r.resolve(rawRef)
	if err != nil {
		return err
	}
	return provider.ShellEnvironment(ctx, ref)
}

func (r *Router) DeleteEnvironment(ctx context.Context, rawRef string) error {
	provider, ref, err := r.resolve(rawRef)
	if err != nil {
		return err
	}
	return provider.DeleteEnvironment(ctx, ref)
}

func (r *Router) InspectEnvironment(ctx context.Context, rawRef string) (core.EnvironmentRuntimeStatus, error) {
	provider, ref, id, err := r.resolveWithID(rawRef)
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	inspector, ok := provider.(InspectorProvider)
	if !ok {
		return core.EnvironmentRuntimeStatus{}, fmt.Errorf("environment provider %q status: %w", id, core.ErrUnsupported)
	}
	return inspector.InspectEnvironment(ctx, ref)
}

func (r *Router) ForwardLocalPort(ctx context.Context, rawRef string, req core.LocalPortRequest) (core.ClientConnection, error) {
	provider, ref, id, err := r.resolveWithID(rawRef)
	if err != nil {
		return core.ClientConnection{}, err
	}
	ports, ok := provider.(LocalPortProvider)
	if !ok {
		return core.ClientConnection{}, fmt.Errorf("environment provider %q local ports: %w", id, core.ErrUnsupported)
	}
	return ports.ForwardLocalPort(ctx, ref, req)
}

func (r *Router) RemoveClientConnection(ctx context.Context, rawRef, connectionID string) error {
	provider, ref, id, err := r.resolveWithID(rawRef)
	if err != nil {
		return err
	}
	ports, ok := provider.(LocalPortProvider)
	if !ok {
		return fmt.Errorf("environment provider %q local ports: %w", id, core.ErrUnsupported)
	}
	return ports.RemoveClientConnection(ctx, ref, connectionID)
}

// PrepareSSH preserves the v0.3 access contract used by older clients.
func (r *Router) PrepareSSH(ctx context.Context, rawRef string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	provider, ref, id, err := r.resolveWithID(rawRef)
	if err != nil {
		return core.ClientConnection{}, err
	}
	if ssh, ok := provider.(SSHProvider); ok {
		return ssh.PrepareSSH(ctx, ref, req)
	}
	if ssh, ok := provider.(SSHAccessProvider); ok {
		return ssh.PrepareSSHAccess(ctx, ref, req)
	}
	return core.ClientConnection{}, fmt.Errorf("environment provider %q SSH: %w", id, core.ErrUnsupported)
}

// PrepareSSHAccess and RevokeSSHAccess satisfy the hardened client contract on
// current main without making SSH a required EnvironmentProvider capability.
func (r *Router) PrepareSSHAccess(ctx context.Context, rawRef string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	provider, ref, id, err := r.resolveWithID(rawRef)
	if err != nil {
		return core.ClientConnection{}, err
	}
	if ssh, ok := provider.(SSHAccessProvider); ok {
		return ssh.PrepareSSHAccess(ctx, ref, req)
	}
	if ssh, ok := provider.(SSHProvider); ok {
		return ssh.PrepareSSH(ctx, ref, req)
	}
	return core.ClientConnection{}, fmt.Errorf("environment provider %q SSH: %w", id, core.ErrUnsupported)
}

func (r *Router) RevokeSSHAccess(ctx context.Context, rawRef, connectionID string) error {
	provider, ref, id, err := r.resolveWithID(rawRef)
	if err != nil {
		return err
	}
	ssh, ok := provider.(SSHAccessProvider)
	if !ok {
		return fmt.Errorf("environment provider %q SSH revoke: %w", id, core.ErrUnsupported)
	}
	return ssh.RevokeSSHAccess(ctx, ref, connectionID)
}

func (r *Router) ListClientConnections(ctx context.Context, rawRef string) ([]core.ClientConnection, error) {
	provider, ref, id, err := r.resolveWithID(rawRef)
	if err != nil {
		return nil, err
	}
	connections, ok := provider.(ConnectionListProvider)
	if !ok {
		return nil, fmt.Errorf("environment provider %q connections: %w", id, core.ErrUnsupported)
	}
	return connections.ListClientConnections(ctx, ref)
}

func (r *Router) resolve(rawRef string) (Provider, string, error) {
	provider, ref, _, err := r.resolveWithID(rawRef)
	return provider, ref, err
}

func (r *Router) resolveWithID(rawRef string) (Provider, string, string, error) {
	id, ref, err := decodeRouteRef(rawRef)
	if err != nil {
		return nil, "", "", err
	}
	provider, err := r.provider(id)
	if err != nil {
		return nil, "", "", err
	}
	return provider, ref, id, nil
}

func (r *Router) provider(id string) (Provider, error) {
	if r == nil {
		return nil, core.ErrRuntimeUnavailable
	}
	provider, ok := r.providers[id]
	if !ok {
		return nil, fmt.Errorf("environment provider %q: %w", id, core.ErrUnsupported)
	}
	return provider, nil
}

func encodeRouteRef(provider, ref string) string {
	return refPrefix + provider + ":" + base64.RawURLEncoding.EncodeToString([]byte(ref))
}

func decodeRouteRef(raw string) (string, string, error) {
	if !strings.HasPrefix(raw, refPrefix) {
		// Pre-v0.7 persisted environments are Incus-backed. Keeping this fallback
		// makes the provider seam migration compatible with existing local state.
		if strings.TrimSpace(raw) == "" {
			return "", "", core.ErrIncompatibleState
		}
		return ProviderIncus, raw, nil
	}
	rest := strings.TrimPrefix(raw, refPrefix)
	cut := strings.IndexByte(rest, ':')
	if cut <= 0 || cut == len(rest)-1 {
		return "", "", core.ErrIncompatibleState
	}
	provider := rest[:cut]
	decoded, err := base64.RawURLEncoding.DecodeString(rest[cut+1:])
	if err != nil || len(decoded) == 0 {
		return "", "", core.ErrIncompatibleState
	}
	return provider, string(decoded), nil
}

type DisabledProvider struct {
	ID     string
	Reason string
}

func (p DisabledProvider) blocked() error {
	reason := p.Reason
	if reason == "" {
		reason = "environment provider is disabled"
	}
	return fmt.Errorf("%s: %w", reason, core.ErrPolicyDenied)
}

func (p DisabledProvider) CreateEnvironment(context.Context, core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	return core.EnvironmentRuntime{}, p.blocked()
}
func (p DisabledProvider) ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error) {
	return core.ExecutionResult{}, p.blocked()
}
func (p DisabledProvider) ShellEnvironment(context.Context, string) error  { return p.blocked() }
func (p DisabledProvider) DeleteEnvironment(context.Context, string) error { return p.blocked() }
func (p DisabledProvider) InspectEnvironment(context.Context, string) (core.EnvironmentRuntimeStatus, error) {
	return core.EnvironmentRuntimeStatus{}, p.blocked()
}
