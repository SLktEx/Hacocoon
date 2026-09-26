package hostsetup

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestStagesFailureCancellationAndNoRawOutput(t *testing.T) {
	for _, failure := range []error{errors.New("SECRET-backend-output"), context.Canceled, context.DeadlineExceeded, ErrNativeBinfmtIncompatible} {
		var events []Event
		ctx := Observe(context.Background(), func(e Event) { events = append(events, e) })
		err := Step(ctx, "wsl_interop", func() error { return failure })
		stage, reason := Details(err)
		if stage != "wsl_interop" || reason != Reason(failure) || !errors.Is(err, failure) {
			t.Fatal(stage, reason, err)
		}
		if len(events) != 2 || events[0].State != "running" || events[1].State != "failed" || strings.Contains(err.Error(), "SECRET") {
			t.Fatal(events, err)
		}
	}
	var events []Event
	ctx, cancel := context.WithCancel(Observe(context.Background(), func(e Event) { events = append(events, e) }))
	err := Step(ctx, "project", func() error { cancel(); return nil })
	if !errors.Is(err, context.Canceled) || events[1].State != "failed" {
		t.Fatal(err, events)
	}
	calls := 0
	_ = Step(ctx, "storage", func() error { calls++; return nil })
	if calls != 0 {
		t.Fatal("called after cancellation")
	}
}

func TestHostToolsStageIsPreserved(t *testing.T) {
	var events []Event
	ctx := Observe(context.Background(), func(e Event) { events = append(events, e) })
	if err := Step(ctx, "host_tools", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Stage != "host_tools" || events[1].Stage != "host_tools" || events[0].State != "running" || events[1].State != "succeeded" {
		t.Fatalf("events=%+v", events)
	}
}

func TestUnknownChildStageCannotBecomeSetup(t *testing.T) {
	for _, failure := range []error{nil, errors.New("SECRET-backend-output")} {
		var events []Event
		ctx := Observe(context.Background(), func(e Event) { events = append(events, e) })
		err := Step(ctx, "setup", func() error {
			return Step(ctx, "SECRET-unregistered-stage", func() error { return failure })
		})
		if !errors.Is(err, failure) {
			t.Fatal(err)
		}
		if len(events) != 4 || events[0].Stage != "setup" || events[3].Stage != "setup" {
			t.Fatalf("events=%+v", events)
		}
		for _, event := range events[1:3] {
			if event.Stage != "unknown" || !ValidStage(event.Stage) {
				t.Fatalf("unknown child impersonates setup or exposes raw stage: %+v", event)
			}
		}
		wantState := "succeeded"
		if failure != nil {
			wantState = "failed"
			stage, reason := Details(err)
			if stage != "unknown" || reason != "failed" || strings.Contains(err.Error(), "SECRET") {
				t.Fatal(stage, reason, err)
			}
		}
		if events[1].State != "running" || events[2].State != wantState || events[3].State != wantState {
			t.Fatalf("events=%+v", events)
		}
	}
}

func TestNotificationFailureDoesNotExposeUnknownOperation(t *testing.T) {
	failure := &NotificationServiceFailure{Operation: "SECRET-backend-command"}
	if failure.Error() != "failed" || Reason(failure) != "failed" {
		t.Fatal("unknown operation became public", failure)
	}
	for _, operation := range []string{"enable_state", "activity", "disable", "reload", "failure_state", "reset", "enable", "restart"} {
		failure := &NotificationServiceFailure{Operation: operation}
		var events []Event
		ctx := Observe(context.Background(), func(e Event) { events = append(events, e) })
		err := Step(ctx, "notification_setup", func() error { return failure })
		stage, reason := Details(err)
		if stage != "notification_setup" || reason != failure.reason() || !ValidReason(reason) || len(events) != 2 || events[1].State != "failed" || events[1].Reason != reason {
			t.Fatal("lost notification operation", stage, reason, events)
		}
	}
}
