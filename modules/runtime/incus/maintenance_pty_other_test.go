//go:build !linux

package incus

import (
	"os"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func openMaintenanceTestPTY() (*os.File, *os.File, error) {
	return nil, nil, core.ErrUnsupported
}
