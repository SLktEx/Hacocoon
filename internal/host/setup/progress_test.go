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
