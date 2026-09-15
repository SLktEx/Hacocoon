package incus

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
)

func TestNotificationSetupHelperFailuresStayClosedAndSpecific(t *testing.T) {
	for code, operation := range map[int]string{50: "enable_state", 51: "activity", 52: "disable", 53: "reload", 54: "failure_state", 55: "reset", 56: "enable", 57: "restart"} {
		for _, commandErr := range []error{nil, errors.New("SECRET-command-output")} {
			result := host.Result{ExitCode: code, Stdout: "SECRET", Stderr: "SECRET"}
			err := hostsetup.Step(context.Background(), "notification_setup", func() error {
				return wslInteropSetupResult("--notifications=refresh", result, commandErr)
			})
			stage, reason := hostsetup.Details(err)
			if err == nil || stage != "notification_setup" || reason != "notification_"+operation+"_failed" || !hostsetup.ValidReason(reason) || strings.Contains(err.Error(), "SECRET") {
				t.Fatal(code, stage, reason, err)
			}
			other := wslInteropSetupResult("", result, commandErr)
			var classified *hostsetup.NotificationServiceFailure
			if other == nil || errors.As(other, &classified) {
				t.Fatal("notification status accepted in unrelated helper mode")
			}
		}
	}
	for _, code := range []int{-1, 1, 49, 58, 255} {
		err := wslInteropSetupResult("--notifications=refresh", host.Result{ExitCode: code}, nil)
		var classified *hostsetup.NotificationServiceFailure
		if err == nil || errors.As(err, &classified) {
			t.Fatal("unknown failure accepted/classified", code, err)
		}
	}
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		if err := wslInteropSetupResult("--notifications=refresh", host.Result{ExitCode: 57}, cause); !errors.Is(err, cause) {
			t.Fatal("cancellation lost", err)
		}
	}
	if err := wslInteropSetupResult("", host.Result{ExitCode: 42}, nil); !errors.Is(err, hostsetup.ErrNativeBinfmtIncompatible) {
		t.Fatal("native interop failure lost", err)
	}
	if err := wslInteropSetupResult("--notifications=refresh", host.Result{}, nil); err != nil {
		t.Fatal(err)
	}
}
