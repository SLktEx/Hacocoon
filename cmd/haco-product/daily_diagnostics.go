package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
	"golang.org/x/term"
	"io"
	"os"
)

func dailyFailure(out io.Writer, operation, stage, name string, err error) int {
	reason := hostsetup.Reason(err)
	var status *control.StatusError
	if errors.As(err, &status) {
		switch status.Code {
		case "not_found", "already_exists", "invalid_argument", "unsupported", "unavailable", "denied", "busy", "incompatible_state", "recovery_required":
			reason = status.Code
		}
	}
	if errors.Is(err, control.ErrUnavailable) {
		reason = "unavailable"
	}
	if errors.Is(err, context.Canceled) {
		reason = "canceled"
	}
	fmt.Fprintf(out, "[failed] operation=%s stage=%s reason=%s\n", operation, stage, reason)
	fmt.Fprintln(out, cliLanguage().Text("daily.unknown_state"))
	if !configEnvironmentName.MatchString(name) {
		name = "<name>"
	}
	fmt.Fprintf(out, cliLanguage().Text("daily.inspect"), name, name)
	if stage == "ssh_connection" && reason == "failed" {
		fmt.Fprintln(out, cliLanguage().Text("daily.ssh_policy"))
	}
	fmt.Fprintln(out, cliLanguage().Text("daily.journal"))
	return 1
}

// Production stdin can be a pipe/FIFO whose writer stays open. Never wait on it
// for a destructive confirmation. In-memory readers support component callers.
func requireInteractiveConfirmation(in io.Reader, diagnostic io.Writer) bool {
	if !interactiveInput(in) {
		fmt.Fprintln(diagnostic, cliLanguage().Text("daily.confirmation"))
		return false
	}
	return true
}

func interactiveInput(in io.Reader) bool {
	file, ok := in.(*os.File)
	return !ok || term.IsTerminal(int(file.Fd()))
}
