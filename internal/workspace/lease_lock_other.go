//go:build !linux

package workspace

import (
	"context"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var fallbackWorkspaceLock sync.Mutex

func lockWorkspace(_ context.Context, _ core.WorkspaceID) (func(), error) {
	fallbackWorkspaceLock.Lock()
	return fallbackWorkspaceLock.Unlock, nil
}
