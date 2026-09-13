// Package hostsetup contains the bounded diagnostic vocabulary for Host setup.
// It grants no authority and does not own resource lifecycle or cleanup.
package hostsetup

import (
	"context"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/core"
	"time"
)

type Event struct {
	Stage      string `json:"stage"`
	State      string `json:"state"`
	Reason     string `json:"reason,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}

type observerKey struct{}

func Observe(ctx context.Context, report func(Event)) context.Context {
	return context.WithValue(ctx, observerKey{}, report)
}
func emit(ctx context.Context, e Event) {
	if report, ok := ctx.Value(observerKey{}).(func(Event)); ok {
		report(e)
	}
}
func ValidStage(s string) bool {
	switch s {
	case "setup", "client_validation", "project", "storage", "copy_recovery", "trusted_host_inspect", "trusted_host_create", "trusted_host_network", "controller_endpoint", "trusted_host_start", "host_tools", "wsl_interop", "client_mode", "client_provision", "host_storage", "official_bases", "host_packages", "host_tooling", "host_services", "notification_setup", "customization":
		return true
	}
	return false
}
func ValidReason(s string) bool {
	switch s {
	case "", "failed", "canceled", "timeout", "not_found", "incompatible_state", "recovery_required", "busy", "denied", "unavailable", "invalid_argument", "unsupported", "native_binfmt_incompatible":
		return true
	}
	return false
}

var ErrNativeBinfmtIncompatible = errors.New("native WSL binfmt registration incompatible")

func Reason(err error) string {
	switch {
	case errors.Is(err, ErrNativeBinfmtIncompatible):
		return "native_binfmt_incompatible"
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, core.ErrRecoveryRequired):
		return "recovery_required"
	case errors.Is(err, core.ErrIncompatibleState):
		return "incompatible_state"
	case errors.Is(err, core.ErrNotFound):
		return "not_found"
	case errors.Is(err, core.ErrWorkspaceBusy), errors.Is(err, core.ErrStorageBusy):
		return "busy"
	case errors.Is(err, core.ErrPolicyDenied), errors.Is(err, core.ErrApprovalDenied):
		return "denied"
	case errors.Is(err, core.ErrRuntimeUnavailable), errors.Is(err, core.ErrStorageUnavailable):
		return "unavailable"
	case errors.Is(err, core.ErrInvalidArgument):
		return "invalid_argument"
	case errors.Is(err, core.ErrUnsupported):
		return "unsupported"
	default:
		return "failed"
	}
}

type Failure struct {
	Stage, Reason string
	cause         error
}

func (e *Failure) Error() string {
	return fmt.Sprintf("Host setup failed: stage=%s reason=%s", e.Stage, e.Reason)
}
func (e *Failure) Unwrap() error { return e.cause }
func Details(err error) (string, string) {
	var f *Failure
	if errors.As(err, &f) && ValidStage(f.Stage) && ValidReason(f.Reason) && f.Reason != "" {
		return f.Stage, f.Reason
	}
	return "setup", Reason(err)
}

// Track surrounds an existing synchronous stage. The caller must use a named
// error return. No success is emitted after cancellation or partial failure.
func Track(ctx context.Context, stage string) func(*error) {
	if !ValidStage(stage) {
		stage = "setup"
	}
	started := time.Now()
	emit(ctx, Event{Stage: stage, State: "running"})
	return func(err *error) {
		e := Event{Stage: stage, State: "succeeded", DurationMS: time.Since(started).Milliseconds()}
		if *err != nil {
			e.State, e.Reason = "failed", Reason(*err)
			var existing *Failure
			if !errors.As(*err, &existing) {
				*err = &Failure{Stage: stage, Reason: e.Reason, cause: *err}
			}
		}
		emit(ctx, e)
	}
}
func Step(ctx context.Context, stage string, fn func() error) (err error) {
	defer Track(ctx, stage)(&err)
	if err = ctx.Err(); err != nil {
		return err
	}
	err = fn()
	if err == nil {
		err = ctx.Err()
	}
	return err
}
