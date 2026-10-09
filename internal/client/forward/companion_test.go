package clientforward

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/logging"
)

func TestCompanionFailureLoggingHasOnlyBoundedObservations(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{errors.New("PRIVATE path argv output"), "other"},
		{fmt.Errorf("PRIVATE: %w", exec.ErrWaitDelay), "wait_delay"},
		{fmt.Errorf("PRIVATE: %w", context.DeadlineExceeded), "timeout"},
		{fmt.Errorf("PRIVATE: %w", context.Canceled), "canceled"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			var out bytes.Buffer
			logger, err := logging.New(logging.Config{Writer: &out, Format: logging.FormatJSON})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(logging.WithLogger(context.Background(), logger))
			cancel()
			recordCompanionFailure(ctx, "wait", tc.err, 12*time.Millisecond)
			var fields map[string]any
			if err := json.Unmarshal(out.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			for key, want := range map[string]any{"level": "ERROR", "component": "client", "operation": "windows_tunnel_companion", "stage": "wait", "reason": tc.want, "context_state": "canceled", "duration_ms": float64(12)} {
				if fields[key] != want {
					t.Errorf("%s=%v, want %v", key, fields[key], want)
				}
			}
			if strings.Contains(out.String(), "PRIVATE") || len(fields) != 9 {
				t.Fatal(out.String())
			}
		})
	}
}

func TestCompanionStartFailurePreservesResultAndOmitsProcessDetails(t *testing.T) {
	var out, diagnostic bytes.Buffer
	logger, err := logging.New(logging.Config{Writer: &diagnostic, Format: logging.FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	ctx := logging.WithLogger(context.Background(), logger)
	cmd := exec.CommandContext(ctx, filepath.Join(t.TempDir(), "PRIVATE-executable"), "PRIVATE-argument")
	code, err := runCompanion(ctx, cmd, testDelegation(), &out, &diagnostic)
	if code != 1 || err == nil || !strings.Contains(err.Error(), "PRIVATE-executable") || cmd.Process != nil {
		t.Fatal("start failure result changed or child unexpectedly launched")
	}
	var fields map[string]any
	if err := json.Unmarshal(diagnostic.Bytes(), &fields); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || len(fields) != 9 || fields["stage"] != "start" || fields["reason"] != "other" || fields["context_state"] != "active" || strings.Contains(diagnostic.String(), "PRIVATE") {
		t.Fatal("companion diagnostic exposed process details or changed output")
	}
}
