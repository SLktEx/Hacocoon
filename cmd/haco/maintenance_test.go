package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestHandleMaintenanceArgsIgnoresOtherCommands(t *testing.T) {
	handled, err := handleMaintenanceArgs([]string{"env", "list"})
	if handled || err != nil {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
}

func TestHandleMaintenanceArgsRejectsInvalidShape(t *testing.T) {
	for _, args := range [][]string{{"maintenance"}, {"maintenance", "unknown"}, {"maintenance", "compact", "extra"}} {
		handled, err := handleMaintenanceArgs(args)
		if !handled || !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("args=%v handled=%t err=%v", args, handled, err)
		}
	}
}

func TestHandleMaintenanceCompactRequiresWindowsHostLauncher(t *testing.T) {
	handled, err := handleMaintenanceArgs([]string{"maintenance", "compact"})
	if !handled || !errors.Is(err, core.ErrUnsupported) {
		t.Fatalf("handled=%t err=%v", handled, err)
	}
	for _, required := range []string{"Windows PowerShell or cmd.exe", "Windows launcher", "fully offline"} {
		if !strings.Contains(err.Error(), required) {
			t.Fatalf("error missing %q: %v", required, err)
		}
	}
}
