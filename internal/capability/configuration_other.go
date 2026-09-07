//go:build !linux

package capability

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (*PolicyConfiguration) Snapshot(context.Context) (PolicySnapshot, error) {
	return PolicySnapshot{}, core.ErrUnsupported
}
func (*PolicyConfiguration) Replace(context.Context, PolicySnapshot) (PolicySnapshot, error) {
	return PolicySnapshot{}, core.ErrUnsupported
}
