package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cliui"
)

// Automatic detection belongs to the local interactive Windows/WSL entry, not
// the controller, generic catalog selector, or ordinary Environment execution.
func hostEntryLanguage(ctx context.Context, getenv func(string) string, detect func(context.Context) (string, error)) cliui.Language {
	if getenv("HACO_UI_LANGUAGE") != "" || getenv("WSL_DISTRO_NAME") == "" {
		return cliui.Resolve(getenv)
	}
	if value, err := detect(ctx); err == nil {
		switch value {
		case "ja":
			return cliui.Japanese
		case "en":
			return cliui.English
		}
	}
	// A missing Windows mount, interop failure or malformed reply must never
	// prevent Host entry or change locale, permission, or controller state.
	return cliui.Resolve(getenv)
}

const windowsLanguageProgram = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"
const windowsLanguageScript = `if ([Globalization.CultureInfo]::CurrentUICulture.TwoLetterISOLanguageName -eq 'ja') { [Console]::Write('ja') } else { [Console]::Write('en') }`

func detectWindowsLanguage(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	// Use the fixed system executable, never a workspace/PATH-selected script.
	cmd := exec.CommandContext(ctx, windowsLanguageProgram, "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", windowsLanguageScript)
	cmd.Dir = "/"
	cmd.Env = []string{"PATH=/usr/bin:/bin", "WSL_INTEROP=" + os.Getenv("WSL_INTEROP"), "WSL_DISTRO_NAME=" + os.Getenv("WSL_DISTRO_NAME")}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = 250 * time.Millisecond
	var output languageReply
	cmd.Stdout = &output
	if err := cmd.Run(); err != nil {
		return "", err
	}
	value := output.value.String()
	if value != "ja" && value != "en" {
		return "", errors.New("unsupported presentation reply")
	}
	return value, nil
}

type languageReply struct{ value strings.Builder }

func (r *languageReply) Write(p []byte) (int, error) {
	if len(p) > 2-r.value.Len() {
		return 0, errors.New("oversized presentation reply")
	}
	return r.value.Write(p)
}
