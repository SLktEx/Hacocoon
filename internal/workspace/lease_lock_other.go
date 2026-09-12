//go:build !linux

package workspace

import (
	"context"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Non-Linux clients do not run the Incus controller. Keep local test lifecycle
// calls cancellable and keyed; the Linux controller uses cross-process locks.
var lifecycleLocks sync.Map

func lockWorkspace(ctx context.Context, id core.WorkspaceID) (func(), error) {
	return lockLifecycle(ctx, "workspace", string(id))
}
func lockLifecycle(ctx context.Context, domain, id string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, _ := lifecycleLocks.LoadOrStore(domain+":"+id, make(chan struct{}, 1))
	gate := value.(chan struct{})
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
