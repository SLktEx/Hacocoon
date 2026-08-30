package host

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/logging"
)

func TestExecRunnerDebugLoggingSanitizesArguments(t *testing.T) {
	var buffer bytes.Buffer
	logger, err := logging.New(logging.Config{Writer: &buffer, Level: slog.LevelDebug, Format: logging.FormatText})
	if err != nil {
		t.Fatal(err)
	}
	ctx := logging.WithLogger(context.Background(), logger)

	_, err = (ExecRunner{}).Run(ctx, "sh", "-c", "exit 0", "--token", "host-command-secret")
	if err != nil {
		t.Fatal(err)
	}
	out := buffer.String()
	if strings.Contains(out, "host-command-secret") {
		t.Fatalf("command secret leaked: %s", out)
	}
	for _, want := range []string{"executing host command", "host command completed", "component=host", "duration_ms="} {
		if !strings.Contains(out, want) {
			t.Fatalf("host command log missing %q: %s", want, out)
		}
	}
}

func TestCommandComponentClassifiesSubsystems(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "incus", args: []string{"launch"}, want: "incus"},
		{name: "incus", args: []string{"network", "show"}, want: "network"},
		{name: "incus", args: []string{"storage", "show"}, want: "storage"},
		{name: "git", args: []string{"status"}, want: "git"},
		{name: "nerdctl", args: []string{"images"}, want: "oci"},
		{name: "nft", args: []string{"list", "ruleset"}, want: "network"},
	}
	for _, test := range tests {
		if got := commandComponent(test.name, test.args); got != test.want {
			t.Fatalf("commandComponent(%q, %v)=%q want %q", test.name, test.args, got, test.want)
		}
	}
}
