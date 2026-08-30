package main

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

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
