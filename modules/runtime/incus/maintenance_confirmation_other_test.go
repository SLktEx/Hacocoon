//go:build !linux

package incus

import (
	"errors"
	"os"
)

func maintenanceConfirmationInput(string) (*os.File, func(), error) {
	return nil, nil, errors.New("native Incus maintenance confirmation requires a Linux terminal")
}
