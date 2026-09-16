//go:build !linux

package oci

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (*MaintenanceTooling) Prepare(context.Context) (string, func() error, error) {
	return "", nil, core.ErrUnsupported
}
