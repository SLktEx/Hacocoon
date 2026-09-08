//go:build !linux

package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func lockHostOperation(context.Context, string) (func(), error) { return nil, core.ErrUnsupported }
