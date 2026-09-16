package cli

import (
	"context"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
	"golang.org/x/term"
	"io"
	"os"
)

func dailyFailure(out io.Writer, operation, stage, name string, err error) int {
	reason := dailyFailureReason(err)
	fmt.Fprintf(out, "[failed] operation=%s stage=%s reason=%s\n", operation, stage, reason)
	_, _ = fmt.Fprintln(out, cliLanguage().Text("daily.unknown_state"))
	if !configEnvironmentName.MatchString(name) {
		name = "<name>"
	}
	_, _ = fmt.Fprintf(out, cliLanguage().Text("daily.inspect"), name, name)
	if stage == "ssh_connection" && reason == "failed" {
		_, _ = fmt.Fprintln(out, cliLanguage().Text("daily.ssh_policy"))
	}
	_, _ = fmt.Fprintln(out, cliLanguage().Text("daily.journal"))
	return 1
}

func dailyFailureReason(err error) string {
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
	return reason
}

// Production stdin can be a pipe/FIFO whose writer stays open. Never wait on it
// for a destructive confirmation. In-memory readers support component callers.
func requireInteractiveConfirmation(in io.Reader, diagnostic io.Writer) bool {
	if !interactiveInput(in) {
		_, _ = fmt.Fprintln(diagnostic, cliLanguage().Text("daily.confirmation"))
		return false
	}
	return true
}

func interactiveInput(in io.Reader) bool {
	file, ok := in.(*os.File)
	return !ok || term.IsTerminal(int(file.Fd()))
}
