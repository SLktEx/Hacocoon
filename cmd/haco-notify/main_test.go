package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
		Events:     []interaction.Event{{EventID: "req:operation-failed", Kind: interaction.OperationFailed, Environment: "dev", NextOffset: 20}},
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
	presenter := &recordingNotifier{}
	for _, test := range []struct {
		name      string
		reader    batchReader
		notifier  notifier
		statePath string
		poll      time.Duration
	}{
		{name: "nil reader", notifier: presenter, statePath: "state.json", poll: time.Second},
		{name: "nil notifier", reader: reader, statePath: "state.json", poll: time.Second},
		{name: "empty state path", reader: reader, notifier: presenter, poll: time.Second},
		{name: "poll too short", reader: reader, notifier: presenter, statePath: "state.json", poll: 100 * time.Millisecond},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := runNative(context.Background(), test.reader, test.notifier, test.statePath, test.poll, true, false); !errors.Is(err, interaction.ErrInvalidArgument) {
				t.Fatalf("expected invalid argument, got %v", err)
			}
		})
	}
}

type recordingReviewNotifier struct {
	recordingNotifier
	requests []string
}

func (n *recordingReviewNotifier) NotifyReview(ctx context.Context, title, body, id string) error {
	n.requests = append(n.requests, id)
	return n.Notify(ctx, title, body)
}
func TestOnlyApprovalEventsCarryReviewCorrelation(t *testing.T) {
	id := strings.Repeat("b", 32)
	reader := &scriptedReader{batch: interaction.Batch{Events: []interaction.Event{
		{EventID: id + ":approval-required", RequestID: id, Kind: interaction.ApprovalRequired, NextOffset: 1},
		{EventID: id + ":operation-failed", RequestID: id, Kind: interaction.OperationFailed, NextOffset: 2},
	}, NextOffset: 2}}
	n := &recordingReviewNotifier{}
	if err := runNative(context.Background(), reader, n, filepath.Join(t.TempDir(), "state"), time.Second, true, false); err != nil {
		t.Fatal(err)
	}
	if len(n.requests) != 1 || n.requests[0] != id || len(n.titles) != 2 {
		t.Fatal(n.requests, n.titles)
	}
}
func TestWindowsReviewActivationIsOnlyAnExactRegisteredRequest(t *testing.T) {
	id := strings.Repeat("b", 32)
	script := windowsReviewToastScript("title", "body", "Hacocoon-Test", id)
	// The Windows harness can parse this exact generated script with its native
	// PowerShell parser; keep it tied to the production generator.
	if output := os.Getenv("HACO_TEST_NATIVE_SCRIPT"); output != "" {
		f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = f.WriteString(script); err != nil {
			t.Fatal(err)
		}
		if err = f.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(script, "$ErrorActionPreference='Stop'") || !strings.Contains(script, "$toast.Tag='"+id[:16]+"'") {
		t.Fatal("notification errors and per-request identity must be preserved")
	}

	if !strings.Contains(script, "://request/"+id) || !strings.Contains(script, "HacocoonDistribution") || !strings.Contains(script, "activationType','protocol") {
		t.Fatal("missing registered request activation")
	}
	for _, pair := range [][2]string{{"Hacocoon-Test", id + "?yes"}, {"-x", id}, {"Hacocoon-Test", ""}} {
		if got := windowsReviewToastScript("title", "body", pair[0], pair[1]); strings.Contains(got, "activationType") {
			t.Fatal("invalid activation was emitted")
		}
	}
	if strings.Contains(script, "approve --") || strings.Contains(script, "answer=") {
		t.Fatal("notification must not carry a decision")
	}
}

func TestNativeNotificationDoesNotExposeSubprocessOutput(t *testing.T) {
	if os.Getenv("HACO_NOTIFICATION_TEST_CHILD") == "1" {
		fmt.Fprintln(os.Stdout, "private-test-output")
		fmt.Fprintln(os.Stderr, "private-test-output")
		os.Exit(7)
	}
	n := commandNotifier{command: func(ctx context.Context, _, _ string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestNativeNotificationDoesNotExposeSubprocessOutput$")
		cmd.Env = append(os.Environ(), "HACO_NOTIFICATION_TEST_CHILD=1")
		return cmd
	}}
	err := n.Notify(context.Background(), "title", "body")
	if err == nil || strings.Contains(err.Error(), "private-test-output") {
		t.Fatal("failed notification must return a fixed error")
	}
}

func TestNativeDiagnosticIsBoundedAndCannotPublishFreeText(t *testing.T) {
	b := &nativeDiagnostic{}
	payload := []byte(strings.Repeat("secret", 1000))
	n, err := b.Write(payload)
	if err != nil || n != len(payload) || len(b.data) != 2048 {
		t.Fatal("native diagnostics must be bounded")
	}
	for _, s := range []string{"HACO_NATIVE_FAILURE:unknown:1", "HACO_NATIVE_FAILURE:show:secret", "private"} {
		if nativeFailurePattern.MatchString(s) {
			t.Fatal("unrecognized diagnostic became public")
		}
	}
	if !nativeFailurePattern.MatchString("HACO_NATIVE_FAILURE:show:-2147024809\n") {
		t.Fatal("missing fixed native failure")
	}
}
func TestWindowsNotificationDoesNotBypassNativeInterop(t *testing.T) {
	n := windowsNotifier().(windowsReviewNotifier)
	cmd := n.command(context.Background(), "title", "body")
	if cmd.Args[0] != "powershell.exe" || strings.Contains(strings.Join(cmd.Args, " "), " /init ") {
		t.Fatal("Windows notification must preserve the normal interop boundary")
	}
}
