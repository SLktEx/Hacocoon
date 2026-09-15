package incus

import (
	"context"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
)

func wslInteropSetupResult(mode string, result host.Result, err error) error {
	if err == nil && result.ExitCode == 0 {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if result.ExitCode == 42 {
		return hostsetup.ErrNativeBinfmtIncompatible
	}
	if mode == "--notifications=refresh" {
		// Private exit codes from the installed setup helper, not arbitrary
		// guest output. A known failure remains a failure; it grants no authority.
		operations := map[int]string{50: "enable_state", 51: "activity", 52: "disable", 53: "reload", 54: "failure_state", 55: "reset", 56: "enable", 57: "restart"}
		if operation, ok := operations[result.ExitCode]; ok {
			return &hostsetup.NotificationServiceFailure{Operation: operation}
		}
	}
	if err == nil {
		err = core.ErrRuntimeUnavailable
	}
	return fmt.Errorf("refresh trusted Host Windows access; rerun Windows installer: %w", err)
}
