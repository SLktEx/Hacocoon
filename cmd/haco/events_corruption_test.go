package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/composition"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

func TestEventsCommandPrintsTrustworthyPrefixBeforeCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	content := "" +
		`{"time":"2026-08-29T08:00:00Z","request_id":"before","type":"requested","capability":"github.git","action":"push"}` + "\n" +
		`{"time":"2026-08-29T08:00:01Z","type":` + "\n" +
		`{"time":"2026-08-29T08:00:02Z","request_id":"after","type":"completed","capability":"github.git","action":"push"}` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	app := &composition.App{Events: eventsapp.New(path)}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "text"},
		{name: "json", args: []string{"--json"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := eventsCommandTo(context.Background(), app, tc.args, &out)
			var corruption *eventsapp.AuditCorruptionError
			if !errors.As(err, &corruption) {
				t.Fatalf("expected AuditCorruptionError, got %v", err)
			}
			if corruption.Line != 2 || corruption.Kind != eventsapp.CorruptionMalformedJSON {
				t.Fatalf("corruption=%#v", corruption)
			}
			got := out.String()
			if !strings.Contains(got, "requested") || !strings.Contains(got, "github.git") {
				t.Fatalf("trustworthy prefix was not printed: %q", got)
			}
			if strings.Contains(got, "completed") || strings.Contains(got, "after") {
				t.Fatalf("record after corruption was exposed: %q", got)
			}
		})
	}
}
