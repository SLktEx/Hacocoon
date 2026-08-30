package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/composition"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

func TestEventsCommandJSONResumesFromByteOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	first := `{"time":"2026-08-29T08:00:00Z","request_id":"first","type":"requested"}` + "\n"
	second := `{"time":"2026-08-29T08:00:01Z","request_id":"second","type":"completed"}` + "\n"
	if err := os.WriteFile(path, []byte(first+second), 0o600); err != nil {
		t.Fatal(err)
	}
	app := &composition.App{Events: eventsapp.New(path)}

	var out bytes.Buffer
	err := eventsCommandTo(context.Background(), app, []string{"--json", "--since-offset", fmt.Sprint(len(first))}, &out)
	if err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Contains(got, `"request_id":"first"`) || !strings.Contains(got, `"request_id":"second"`) {
		t.Fatalf("incremental output=%q", got)
	}
	wantOffset := fmt.Sprintf(`"next_offset":%d`, len(first)+len(second))
	if !strings.Contains(got, wantOffset) {
		t.Fatalf("output lacks resume cursor %s: %q", wantOffset, got)
	}
}

func TestEventsCommandRejectsInvalidOffset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte(`{"time":"2026-08-29T08:00:00Z","type":"requested"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	app := &composition.App{Events: eventsapp.New(path)}

	for _, args := range [][]string{
		{"--since-offset", "-1"},
		{"--since-offset", "not-a-number"},
		{"--since-offset"},
		{"--since-offset", "0", "--since-offset", "0"},
		{"--json", "--json"},
	} {
		var out bytes.Buffer
		if err := eventsCommandTo(context.Background(), app, args, &out); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}
