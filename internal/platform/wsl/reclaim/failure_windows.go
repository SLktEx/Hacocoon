//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"errors"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

func PreparationFailure(err error) reclamation.InvocationFailure {
	result := reclamation.InvocationFailure{Phase: "prepare", Stage: "other"}
	var failure *targetFailure
	if errors.As(err, &failure) {
		result.Stage = failure.stage
	}
	var native syscall.Errno
	if errors.As(err, &native) {
		result.NativeError = uint32(native)
	}
	if !result.Valid() {
		result.Stage = "other"
	}
	return result
}
