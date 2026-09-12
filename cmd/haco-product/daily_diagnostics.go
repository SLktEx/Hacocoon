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
	fmt.Fprintln(out, "Completion is not confirmed; resource state is unknown until inspected. Do not assume cleanup or retry succeeded.")
	if !configEnvironmentName.MatchString(name) {
		name = "<name>"
	}
	fmt.Fprintf(out, "Next: haco env status %s; haco doctor %s. Inspect before retrying or deleting retained data.\n", name, name)
	if stage == "ssh_connection" {
		fmt.Fprintln(out, "If the Base lacks sshd, SSH preparation needs package access under the current Env Policy. In trusted haco-host, inspect haco approve --list and haco config; review the package endpoints before changing Policy. No approval or package failure is inferred from this error.")
	}
	fmt.Fprintln(out, "Diagnostics (WSL/Linux Physical Host, administrator): journalctl -u haco-controller.service --since '30 minutes ago' --no-pager")
	return 1
}

// Production stdin can be a pipe/FIFO whose writer stays open. Never wait on it
// for a destructive confirmation. In-memory readers support component callers.
func requireInteractiveConfirmation(in io.Reader, diagnostic io.Writer) bool {
	if !interactiveInput(in) {
		fmt.Fprintln(diagnostic, "No changes made: confirmation requires a terminal; review the target and use --yes for scripts.")
		return false
	}
	return true
}

func interactiveInput(in io.Reader) bool {
	file, ok := in.(*os.File)
	return !ok || term.IsTerminal(int(file.Fd()))
}
