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
	ErrPolicyDenied       = errors.New("capability denied by policy")
	ErrApprovalDenied     = errors.New("capability approval denied")
	ErrAuditIncomplete    = errors.New("capability executed but audit is incomplete")
	ErrIncompatibleState  = errors.New("incompatible state")
	ErrRecoveryRequired   = errors.New("manual recovery required")
	ErrCapabilityStale    = errors.New("capability request no longer matches current state")
)
