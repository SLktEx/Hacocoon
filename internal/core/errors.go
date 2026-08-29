package core

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrUnsupported        = errors.New("unsupported")
	ErrUnsafeShrink       = errors.New("unsafe shrink refused")
	ErrRuntimeUnavailable = errors.New("runtime unavailable")
	ErrStorageUnavailable = errors.New("storage unavailable")
)
