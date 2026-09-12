package main

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

func TestNativeDistinctRequestFailureStormPersistsCursorAndFirstNotice(t *testing.T) {
	reader := &scriptedReader{}
	for i := 0; i < 100; i++ {
		reader.batch.Events = append(reader.batch.Events, interaction.Event{EventID: fmt.Sprintf("req-%d:policy-denied", i), RequestID: fmt.Sprintf("req-%d", i), Kind: interaction.PolicyDenied, Environment: "dev", Capability: "network.connect", Action: "connect", NextOffset: int64(i + 1)})
	}
	reader.batch.NextOffset = 100
	presenter := &recordingNotifier{}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := runNative(context.Background(), reader, presenter, path, time.Second, true, false); err != nil {
		t.Fatal(err)
	}
	if len(presenter.titles) != 1 {
		t.Fatalf("one failure burst produced %d notifications", len(presenter.titles))
	}
	state, err := loadState(path)
	if err != nil || state.Offset != 100 || !state.hasSeen("req-99:policy-denied") {
		t.Fatalf("cursor/event lost: %+v %v", state, err)
	}
	// A new request after restart shares only presentation cooldown, never authority.
	reader.batch.Events = reader.batch.Events[:1]
	reader.batch.Events[0].EventID = "after-restart:policy-denied"
	reader.batch.Events[0].NextOffset = 101
	reader.batch.NextOffset = 101
	if err := runNative(context.Background(), reader, presenter, path, time.Second, true, false); err != nil {
		t.Fatal(err)
	}
	if len(presenter.titles) != 1 {
		t.Fatal("restart replayed the failure burst")
	}
}

func TestFailureNoticeDoesNotHideOtherTargetsApprovalRecoveryOrLaterFailure(t *testing.T) {
	now := time.Now()
	e := interaction.Event{Kind: interaction.OperationFailed, Environment: "dev", Capability: "network.connect", Action: "connect"}
	var state notifyState
	state.rememberFailure(e, now)
	if !state.suppressFailure(e, now.Add(time.Second)) {
		t.Fatal("burst not suppressed")
	}
	for _, change := range []func(*interaction.Event){
		func(e *interaction.Event) { e.Environment = "other" },
		func(e *interaction.Event) { e.Capability = "git.push" },
		func(e *interaction.Event) { e.Code = "different" },
		func(e *interaction.Event) { e.Kind = interaction.ApprovalRequired },
		func(e *interaction.Event) { e.Kind = interaction.RecoveryRequired },
		func(e *interaction.Event) { e.RecoveryRequired = true },
	} {
		other := e
		change(&other)
		if state.suppressFailure(other, now.Add(time.Second)) {
			t.Fatalf("hidden event: %+v", other)
		}
	}
	if state.suppressFailure(e, now.Add(time.Minute)) || state.suppressFailure(e, now.Add(-time.Second)) {
		t.Fatal("later failure or clock rollback hidden")
	}
	for i := 0; i < 1000; i++ {
		e.Environment = fmt.Sprint(i)
		state.rememberFailure(e, now)
	}
	if len(state.Failures) > maxFailureNotices {
		t.Fatal("unbounded presentation state")
	}
}
