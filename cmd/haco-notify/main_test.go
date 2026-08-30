package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

type scriptedReader struct {
	batch       interaction.Batch
	err         error
	seenOffset  int64
	seenLimit   int
	calledCount int
}

func (r *scriptedReader) Batch(_ context.Context, offset int64, limit int) (interaction.Batch, error) {
	r.calledCount++
	r.seenOffset = offset
	r.seenLimit = limit
	return r.batch, r.err
}

type recordingNotifier struct {
	titles []string
	bodies []string
	err    error
}

func (n *recordingNotifier) Notify(_ context.Context, title, body string) error {
	if n.err != nil {
		return n.err
	}
	n.titles = append(n.titles, title)
	n.bodies = append(n.bodies, body)
	return nil
}

func TestNotificationTextUsesOnlyMinimizedFields(t *testing.T) {
	event := interaction.Event{
		Kind:        interaction.ApprovalRequired,
		Environment: "dev",
		Capability:  "git.push",
		Action:      "push",
		Code:        "ignored-for-details",
	}
	title, body, show := notificationText(event, false)
	if !show || title != "Hacocoon approval required" || body != "dev · git.push · push" {
		t.Fatalf("unexpected notification: %q %q %v", title, body, show)
	}

	_, _, show = notificationText(interaction.Event{Kind: interaction.OperationCompleted}, false)
	if show {
		t.Fatal("completed notifications must be opt-in")
	}
}

func TestWindowsToastScriptDoesNotInterpolateUntrustedText(t *testing.T) {
	title := "'); Remove-Item C:\\ -Recurse; #"
	body := "<xml>&'\""
	script := windowsToastScript(title, body)
	if strings.Contains(script, title) || strings.Contains(script, body) {
		t.Fatalf("untrusted notification text was interpolated into PowerShell: %s", script)
	}
	for _, value := range []string{title, body} {
		encoded := base64.StdEncoding.EncodeToString([]byte(value))
		if !strings.Contains(script, encoded) {
			t.Fatalf("encoded payload missing: %q", value)
		}
	}
}

func TestNotifyStateRoundTripAndDedupBound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "notify.json")
	state := notifyState{Offset: 123}
	for i := 0; i < maxSeenEventIDs+20; i++ {
		state.remember(strings.Repeat("x", i%5+1) + string(rune('A'+(i%26))) + string(rune(i+1000)))
	}
	if len(state.SeenEventIDs) > maxSeenEventIDs {
		t.Fatalf("seen IDs grew beyond bound: %d", len(state.SeenEventIDs))
	}
	state.remember("req:approval-required")
	state.remember("req:approval-required")
	if err := saveState(path, state); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("state file is too permissive: %o", info.Mode().Perm())
	}
	loaded, err := loadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Offset != 123 || !loaded.hasSeen("req:approval-required") {
		t.Fatalf("unexpected loaded state: %+v", loaded)
	}
}

func TestRunNativeResumesDeduplicatesAndCommitsCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native.json")
	if err := saveState(path, notifyState{Offset: 10, SeenEventIDs: []string{"req-old:approval-required"}}); err != nil {
		t.Fatal(err)
	}

	reader := &scriptedReader{batch: interaction.Batch{
		SchemaVersion: interaction.SchemaVersion,
		Events: []interaction.Event{
			{EventID: "req-old:approval-required", Kind: interaction.ApprovalRequired, Environment: "old", NextOffset: 20},
			{EventID: "req-new:approval-required", Kind: interaction.ApprovalRequired, Environment: "dev", Capability: "git.push", Action: "push", NextOffset: 30},
			{EventID: "req-new:approval-required", Kind: interaction.ApprovalRequired, Environment: "dev", Capability: "git.push", Action: "push", NextOffset: 40},
		},
		NextOffset: 50,
	}}
	notifier := &recordingNotifier{}
	if err := runNative(context.Background(), reader, notifier, path, time.Second, true, false); err != nil {
		t.Fatal(err)
	}
	if reader.calledCount != 1 || reader.seenOffset != 10 || reader.seenLimit != interaction.DefaultBatchSize {
		t.Fatalf("unexpected reader call: count=%d offset=%d limit=%d", reader.calledCount, reader.seenOffset, reader.seenLimit)
	}
	if len(notifier.titles) != 1 || notifier.titles[0] != "Hacocoon approval required" || notifier.bodies[0] != "dev · git.push · push" {
		t.Fatalf("duplicate/replayed notifications were not suppressed: titles=%v bodies=%v", notifier.titles, notifier.bodies)
	}
	state, err := loadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Offset != 50 || !state.hasSeen("req-old:approval-required") || !state.hasSeen("req-new:approval-required") {
		t.Fatalf("resume state was not committed: %+v", state)
	}
}

func TestRunNativeStopsBeforeCursorAdvanceWhenDeliveryFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native.json")
	if err := saveState(path, notifyState{Offset: 10}); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("desktop unavailable")
	reader := &scriptedReader{batch: interaction.Batch{
		Events: []interaction.Event{{EventID: "req:operation-failed", Kind: interaction.OperationFailed, Environment: "dev", NextOffset: 20}},
		NextOffset: 20,
	}}
	notifier := &recordingNotifier{err: wantErr}
	err := runNative(context.Background(), reader, notifier, path, time.Second, true, false)
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	state, loadErr := loadState(path)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if state.Offset != 10 || state.hasSeen("req:operation-failed") {
		t.Fatalf("failed delivery incorrectly committed state: %+v", state)
	}
}

func TestRunNativeCommitsTrustworthyPrefixThenStopsOnCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "native.json")
	corruption := &interaction.CorruptionError{Line: 4, ByteOffset: 91, Kind: interaction.CorruptionMalformedJSON}
	reader := &scriptedReader{
		batch: interaction.Batch{
			Events:     []interaction.Event{{EventID: "safe:operation-failed", Kind: interaction.OperationFailed, Environment: "dev", NextOffset: 80}},
			NextOffset: 80,
		},
		err: corruption,
	}
	notifier := &recordingNotifier{}
	err := runNative(context.Background(), reader, notifier, path, time.Second, true, false)
	var gotCorruption *interaction.CorruptionError
	if !errors.As(err, &gotCorruption) {
		t.Fatalf("expected corruption error, got %v", err)
	}
	state, loadErr := loadState(path)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(notifier.titles) != 1 || state.Offset != 80 || !state.hasSeen("safe:operation-failed") {
		t.Fatalf("trustworthy prefix was not committed before stop: notifications=%v state=%+v", notifier.titles, state)
	}
}

func TestRunNativeRejectsInvalidDependencies(t *testing.T) {
	reader := &scriptedReader{}
	notifier := &recordingNotifier{}
	for _, test := range []struct {
		name      string
		reader    batchReader
		notifier  notifier
		statePath string
		poll      time.Duration
	}{
		{name: "nil reader", notifier: notifier, statePath: "state.json", poll: time.Second},
		{name: "nil notifier", reader: reader, statePath: "state.json", poll: time.Second},
		{name: "empty state path", reader: reader, notifier: notifier, poll: time.Second},
		{name: "poll too short", reader: reader, notifier: notifier, statePath: "state.json", poll: 100 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := runNative(context.Background(), test.reader, test.notifier, test.statePath, test.poll, true, false); !errors.Is(err, interaction.ErrInvalidArgument) {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}
}
