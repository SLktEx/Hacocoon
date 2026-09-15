//go:build !linux

package state

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
)

// Non-Linux clients do not run the controller. This keyed, cancellable lock
// supports local component tests, not a Windows-native controller.
var lifecycleLocks sync.Map

func (s *EnvironmentJSONStore) LockLifecycle(ctx context.Context, domain, id string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if (domain != "environment" && domain != "workspace") || id == "" {
		return nil, fmt.Errorf("invalid lifecycle lock identity")
	}
	path, err := filepath.Abs(s.path)
	if err != nil {
		return nil, err
	}
	value, _ := lifecycleLocks.LoadOrStore(path+"\x00"+domain+"\x00"+id, make(chan struct{}, 1))
	gate := value.(chan struct{})
	select {
	case gate <- struct{}{}:
		return func() { <-gate }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
