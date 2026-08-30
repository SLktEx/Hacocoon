//go:build !linux

package run

import (
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type runOwnershipLock interface {
	Release() error
}

func acquireOwnershipLock(_, _ string, _ bool) (runOwnershipLock, bool, error) {
	return nil, false, fmt.Errorf("crash-safe ephemeral run ownership requires Linux flock: %w", core.ErrUnsupported)
}
