package gitrepo

import (
	"context"
	"errors"
	"net"
	"os"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const brokerSocketProbeTimeout = 50 * time.Millisecond

// Status inspects only trusted local broker state. It never performs a Git
// operation or contacts a repository remote. applicable is false for managed
// Workspaces that contain no Git route.
func (b *Broker) Status(ctx context.Context, name string) (applicable, connected bool, err error) {
	environment, err := b.Environments.GetEnvironment(ctx, name)
	if err != nil {
		return false, false, err
	}
	if !ValidID(name) || !strings.HasPrefix(environment.Workspace.Path, "managed:") {
		return false, false, core.ErrInvalidArgument
	}
	workspace, err := b.Repositories.Get("work", strings.TrimPrefix(environment.Workspace.Path, "managed:"))
	if err != nil {
		return false, false, err
	}

	bound := binding{Environment: environment, Workspace: workspace}
	for _, member := range workspace.Copies() {
		if member.Remote == "" {
			continue
		}
		applicable = true
		repo, err := b.Repositories.Get("repo", member.Repository)
		if err != nil {
			return true, false, nil
		}
		if repo.Remote != member.Remote || repo.Branch != member.Branch {
			return true, false, nil
		}
		if len(workspace.Members) == 0 {
			bound.Repository = repo
		} else {
			bound.Repositories = append(bound.Repositories, repo)
		}
	}
	if !applicable {
		return false, false, nil
	}
	if err := b.validateBinding(ctx, bound); err != nil {
		if ctx.Err() != nil {
			return true, false, ctx.Err()
		}
		return true, false, nil
	}

	b.mu.Lock()
	brokerRunning := b.ctx != nil && b.ctx.Err() == nil
	current, exists := b.servers[name]
	b.mu.Unlock()
	if !brokerRunning || !exists || !reflect.DeepEqual(current.binding, bound) {
		return true, false, nil
	}
	healthy, _, socketErr := localBrokerSocketState(b.socket(name))
	if socketErr != nil || !healthy {
		return true, false, nil
	}
	return true, true, nil
}

// Repair restores only the trusted local broker listener and guest broker
// attachment. It reuses Connect after making a missing/stale listener eligible
// for recreation, and never performs a repository operation or remote probe.
func (b *Broker) Repair(ctx context.Context, name string) error {
	b.mu.Lock()
	if current, exists := b.servers[name]; exists {
		healthy, replaceable, err := localBrokerSocketState(b.socket(name))
		if err != nil {
			b.mu.Unlock()
			return err
		}
		if !healthy {
			if !replaceable {
				b.mu.Unlock()
				return core.ErrIncompatibleState
			}
			// Close removes only the broker's Unix socket. A non-socket path is
			// rejected above rather than deleted.
			_ = current.server.Close()
			delete(b.servers, name)
		}
	}
	b.mu.Unlock()
	return b.Connect(ctx, name)
}

func localBrokerSocketState(path string) (healthy, replaceable bool, err error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, true, nil
	}
	if err != nil {
		return false, false, err
	}
	if info.Mode()&os.ModeSocket == 0 {
		return false, false, core.ErrIncompatibleState
	}
	conn, err := net.DialTimeout("unix", path, brokerSocketProbeTimeout)
	if err == nil {
		_ = conn.Close()
		return true, false, nil
	}
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ECONNREFUSED) {
		return false, true, nil
	}
	return false, false, err
}
