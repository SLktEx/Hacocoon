package core

import "errors"

var (
	ErrNotFound           = errors.New("not found")
	ErrAlreadyExists      = errors.New("already exists")
	ErrInvalidArgument    = errors.New("invalid argument")
	ErrUnsupported        = errors.New("unsupported")
	ErrUnsafeShrink       = errors.New("unsafe shrink refused")
	ErrRuntimeUnavailable = errors.New("runtime unavailable")
	ErrStorageUnavailable = errors.New("storage unavailable")
	ErrStorageBusy        = errors.New("storage has active sessions")
	ErrWorkspaceBusy      = errors.New("workspace has conflicting active lease")
)
