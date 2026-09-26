package gitrepo

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// ConnectionStatus reports local broker wiring, never upstream reachability or authorization.
type ConnectionStatus struct {
	Configured bool `json:"configured"`
	Connected  bool `json:"connected"`
}

func (b *Broker) ConnectionStatus(ctx context.Context, name string) (ConnectionStatus, error) {
	if !gitadapter.ValidID(name) {
		return ConnectionStatus{}, core.ErrInvalidArgument
	}
	env, err := b.Environments.GetEnvironment(ctx, name)
	if err != nil {
		return ConnectionStatus{}, err
	}
	if !strings.HasPrefix(env.Workspace.Path, "managed:") {
		return ConnectionStatus{}, nil
	}
	bound, err := b.connectionBinding(ctx, name)
	if errors.Is(err, core.ErrUnsupported) {
		return ConnectionStatus{}, nil
	}
	if err != nil {
		return ConnectionStatus{}, err
	}
	result := ConnectionStatus{Configured: true}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx == nil || b.ctx.Err() != nil {
		return result, nil
	}
	entry, exists := b.servers[name]
	if !exists || !reflect.DeepEqual(entry.binding, bound) {
		return result, nil
	}
	info, err := os.Lstat(b.socket(name))
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return result, core.ErrIncompatibleState
	}
	inspector, ok := b.Repositories.Backend.(interface {
		InspectGitConnection(context.Context, core.Environment, Object, string) (bool, error)
	})
	if !ok {
		return result, core.ErrUnsupported
	}
	result.Connected, err = inspector.InspectGitConnection(ctx, bound.Environment, bound.Workspace, b.socket(name))
	return result, err
}
