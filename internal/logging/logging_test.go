package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"":        slog.LevelInfo,
		"debug":   slog.LevelDebug,
		"INFO":    slog.LevelInfo,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
	}
	for raw, want := range tests {
		got, err := ParseLevel(raw)
		if err != nil {
			t.Fatalf("ParseLevel(%q): %v", raw, err)
		}
		if got != want {
			t.Fatalf("ParseLevel(%q)=%v want %v", raw, got, want)
		}
	}
	if _, err := ParseLevel("trace"); err == nil {
		t.Fatal("ParseLevel(trace) unexpectedly succeeded")
	}
}

func TestLoggerFiltersDebugByDefaultLevel(t *testing.T) {
	var buffer bytes.Buffer
	logger, err := New(Config{Writer: &buffer, Level: slog.LevelInfo, Format: FormatText})
	if err != nil {
		t.Fatal(err)
	}
	logger.Debug("hidden")
	logger.Info("visible", "component", "core")
	if strings.Contains(buffer.String(), "hidden") {
		t.Fatalf("debug log was not filtered: %q", buffer.String())
	}
	if !strings.Contains(buffer.String(), "visible") || !strings.Contains(buffer.String(), "component=core") {
		t.Fatalf("info log missing structured field: %q", buffer.String())
	}
}

func TestJSONLoggerPreservesStructuredFields(t *testing.T) {
	var buffer bytes.Buffer
	logger, err := New(Config{Writer: &buffer, Level: slog.LevelDebug, Format: FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("created", "component", "core", "environment_id", "demo", "duration_ms", 12)
	out := buffer.String()
	for _, want := range []string{`"msg":"created"`, `"component":"core"`, `"environment_id":"demo"`, `"duration_ms":12`} {
		if !strings.Contains(out, want) {
			t.Fatalf("JSON log missing %s: %s", want, out)
		}
	}
}

func TestLoggerRedactsSensitiveFieldsAndStrings(t *testing.T) {
	var buffer bytes.Buffer
	logger, err := New(Config{Writer: &buffer, Level: slog.LevelDebug, Format: FormatText})
	if err != nil {
		t.Fatal(err)
	}
	logger.Info("request Authorization: Bearer message-secret",
		"token", "field-secret",
		"remote", "https://user:url-secret@example.com/repo.git?token=query-secret",
	)
	out := buffer.String()
	for _, secret := range []string{"message-secret", "field-secret", "user", "url-secret", "query-secret"} {
		if strings.Contains(out, secret) {
			t.Fatalf("secret %q leaked in log: %s", secret, out)
		}
	}
	if !strings.Contains(out, Redacted) {
		t.Fatalf("redaction marker missing: %s", out)
	}
}

func TestRedactStringCoversTokenUserinfoAndEnvironmentAssignments(t *testing.T) {
	raw := "GH_TOKEN=env-secret remote=https://userinfo-secret@example.com/repo?client_secret=query-secret"
	got := RedactString(raw)
	for _, secret := range []string{"env-secret", "userinfo-secret", "query-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked after redaction: %s", secret, got)
		}
	}
}

func TestSanitizeArgs(t *testing.T) {
	args := []string{
		"--token", "token-secret",
		"--password=password-secret",
		"API_KEY=key-secret",
		"https://userinfo-secret@example.com/repo?access_token=query-secret",
		"--project", "hacocoon",
	}
	got := strings.Join(SanitizeArgs(args), " ")
	for _, secret := range []string{"token-secret", "password-secret", "key-secret", "userinfo-secret", "query-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in sanitized args: %s", secret, got)
		}
	}
	if !strings.Contains(got, "--project hacocoon") {
		t.Fatalf("non-sensitive args changed unexpectedly: %s", got)
	}
}

func TestContextLoggerCarriesOperationFields(t *testing.T) {
	var buffer bytes.Buffer
	logger, err := New(Config{Writer: &buffer, Level: slog.LevelInfo, Format: FormatText})
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithLogger(context.Background(), logger)
	ctx = With(ctx, "operation", "create_environment", "environment_id", "demo")
	FromContext(ctx).InfoContext(ctx, "created", "component", "core")
	out := buffer.String()
	for _, want := range []string{"operation=create_environment", "environment_id=demo", "component=core"} {
		if !strings.Contains(out, want) {
			t.Fatalf("context log missing %q: %s", want, out)
		}
	}
}
