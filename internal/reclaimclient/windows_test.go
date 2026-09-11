package reclaimclient

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

var commandReclaimTarget = reclamation.WSLTarget{RegistrationID: "{11111111-1111-4111-8111-111111111111}", InstallationID: "22222222-2222-4222-8222-222222222222"}

func TestWindowsReclaimScriptHasFixedHelperAndExactGUID(t *testing.T) {
	for _, mode := range []string{"start", "status"} {
		script, err := windowsReclaimScript(commandReclaimTarget, mode)
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range []string{"LocalApplicationData", "Hacocoon\\reclamation\\", "ToString('N')", commandReclaimTarget.RegistrationID} {
			if !strings.Contains(script, required) {
				t.Fatal("missing fixed binding", required)
			}
		}
		if mode == "status" && (strings.Contains(script, "_prepare") || strings.Contains(script, "_launch")) {
			t.Fatal("status mutates")
		}
		if mode == "start" {
			for _, required := range []string{"_prepare", "_launch", "TryParse", `completion\.\s*$`} {
				if !strings.Contains(script, required) {
					t.Fatal("missing dispatch validation", required)
				}
			}
		}
	}
	if _, err := windowsReclaimScript(commandReclaimTarget, "--shutdown"); err == nil {
		t.Fatal("arbitrary action accepted")
	}
	target := commandReclaimTarget
	target.RegistrationID = "';exit 0;#"
	if _, err := windowsReclaimScript(target, "start"); err == nil {
		t.Fatal("injected identity accepted")
	}
	var out reclaimOutput
	if _, err := out.Write(bytes.Repeat([]byte("x"), 16385)); err == nil {
		t.Fatal("unbounded capture")
	}
}

func TestWindowsReviewScriptIsBoundedToObservedOperation(t *testing.T) {
	operation := "{33333333-3333-4333-8333-333333333333}"
	for _, state := range []string{"pending", "failed", "interrupted"} {
		script, err := windowsReviewScript(commandReclaimTarget, operation, state)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(script, "_prepare") || strings.Contains(script, "_launch") || !strings.Contains(script, operation) {
			t.Fatal("review became a new operation")
		}
	}
	for _, request := range []struct{ operation, state string }{{"';exit 0;#", "pending"}, {"{00000000-0000-0000-0000-000000000000}", "pending"}, {operation, "complete"}, {operation, "_launch"}} {
		if _, err := windowsReviewScript(commandReclaimTarget, request.operation, request.state); err == nil {
			t.Fatal("invalid review request accepted")
		}
	}
}
