package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

var commandReclaimTarget = reclamation.WSLTarget{RegistrationID: "{11111111-1111-4111-8111-111111111111}", InstallationID: "22222222-2222-4222-8222-222222222222"}

const commandReclaimOperation = "{33333333-3333-4333-8333-333333333333}"

func TestReclaimPublicCommandUsesNoCallerIdentityAndNeverRetries(t *testing.T) {
	for _, tc := range []struct {
		name                string
		args                []string
		answer              string
		raw                 string
		invokeErr           error
		wantCalls, wantCode int
	}{
		{name: "decline", answer: "n\n", wantCode: 0},
		{name: "confirm", answer: "y\n", raw: `{"operation":"` + commandReclaimOperation + `","worker_pid":42}`, wantCalls: 1},
		{name: "explicit yes", args: []string{"--yes"}, raw: `{"operation":"` + commandReclaimOperation + `","worker_pid":42}`, wantCalls: 1},
		{name: "lost dispatch", args: []string{"--yes"}, invokeErr: errors.New("token=private"), wantCalls: 1, wantCode: 1},
		{name: "bad result", args: []string{"--yes"}, raw: `{"operation":"--shutdown","worker_pid":42}`, wantCalls: 1, wantCode: 1},
		{name: "pending query", args: []string{"--status"}, raw: `{"operation":"` + commandReclaimOperation + `","state":"pending","linux_started":true}`, wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			calls := 0
			code := reclaimCommand(context.Background(), tc.args, strings.NewReader(tc.answer), &out, &diagnostic, func(context.Context) (reclamation.WSLTarget, error) { return commandReclaimTarget, nil }, func(_ context.Context, target reclamation.WSLTarget, mode string) ([]byte, error) {
				calls++
				if target != commandReclaimTarget {
					t.Fatal("changed target")
				}
				if mode != "start" && mode != "status" {
					t.Fatal(mode)
				}
				return []byte(tc.raw), tc.invokeErr
			})
			if code != tc.wantCode || calls != tc.wantCalls || strings.Contains(diagnostic.String(), "private") {
				t.Fatal(code, calls, out.String(), diagnostic.String())
			}
			if tc.name == "pending query" && (!strings.Contains(out.String(), "unknown") || strings.Contains(out.String(), "Continue?")) {
				t.Fatal(out.String())
			}
			if tc.name == "confirm" && !strings.Contains(out.String(), "not yet confirmed") {
				t.Fatal("dispatch claimed completion")
			}
		})
	}
	for _, args := range [][]string{{"foreign"}, {"--status", "--yes"}, {"--yes", "--yes"}, {"--status", "--status"}} {
		if code := reclaimCommand(context.Background(), args, strings.NewReader(""), io.Discard, io.Discard, func(context.Context) (reclamation.WSLTarget, error) {
			t.Fatal("invalid arguments queried controller")
			return commandReclaimTarget, nil
		}, nil); code != 2 {
			t.Fatal(code)
		}
	}
}
func TestReclaimPromptCancellationNeverInvokesWindows(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := reclaimCommand(ctx, nil, reader, io.Discard, io.Discard, func(context.Context) (reclamation.WSLTarget, error) { return commandReclaimTarget, nil }, func(context.Context, reclamation.WSLTarget, string) ([]byte, error) {
		t.Fatal("canceled prompt invoked Windows")
		return nil, nil
	})
	if code != 0 && code != 1 {
		t.Fatal(code)
	}
}
func TestReclamationStatusDoesNotInventCompletionOrAllocation(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		code      int
		contains  string
	}{
		{"pending", `{"operation":"` + commandReclaimOperation + `","state":"pending"}`, 0, "outcome unknown"},
		{"interrupted", `{"operation":"` + commandReclaimOperation + `","state":"interrupted","linux_started":true}`, 1, "outcome unknown"},
		{"failed", `{"operation":"` + commandReclaimOperation + `","state":"failed"}`, 1, "failed"},
		{"unproven", `{"operation":"` + commandReclaimOperation + `","state":"complete"}`, 1, ""},
		{"extra", `{"operation":"` + commandReclaimOperation + `","state":"pending","command":"private"}`, 1, ""},
		{"unknown failure", `{"operation":"` + commandReclaimOperation + `","state":"failed","observation":{"Failure":"token=private"}}`, 1, ""},
		{"native error", `{"operation":"` + commandReclaimOperation + `","state":"failed","observation":{"Failure":"stop","NativeError":5,"StopAttempted":true}}`, 1, "native error: 5"},
		{"inconsistent", `{"operation":"` + commandReclaimOperation + `","state":"failed","observation":{"StopRequested":true}}`, 1, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			code := writeReclamationStatus(&out, &diagnostic, []byte(tc.raw))
			if code != tc.code || !strings.Contains(out.String(), tc.contains) || strings.Contains(out.String(), "reclaimed: 0") {
				t.Fatal(code, out.String(), diagnostic.String())
			}
		})
	}
}

func TestReclamationStatusReportsMeasuredReduction(t *testing.T) {
	raw := `{"operation":"` + commandReclaimOperation + `","state":"complete","observation":{"StopAttempted":true,"StopRequested":true,"ResumeAttempted":true,"Resumed":true,"Compaction":{"Attempted":true,"Completed":true,"OpenAttempts":2,"Virtual":{"Capacity":1048576},"Before":{"LogicalBytes":4096,"AllocatedBytes":2048},"After":{"LogicalBytes":4096,"AllocatedBytes":1024}}}}`
	var out, diagnostic bytes.Buffer
	if code := writeReclamationStatus(&out, &diagnostic, []byte(raw)); code != 0 {
		t.Fatal(code, diagnostic.String())
	}
	for _, want := range []string{"Windows-only operation: complete", "Linux stages: unrecorded", "virtual capacity: 1048576", "reclaimed: 1024 bytes"} {
		if !strings.Contains(out.String(), want) {
			t.Fatal(want, out.String())
		}
	}
}

type failedReclaimWriter struct{}

func (failedReclaimWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestReclaimDoesNotDispatchWhenWarningCannotBeShown(t *testing.T) {
	code := reclaimCommand(context.Background(), []string{"--yes"}, strings.NewReader(""), failedReclaimWriter{}, io.Discard,
		func(context.Context) (reclamation.WSLTarget, error) { return commandReclaimTarget, nil },
		func(context.Context, reclamation.WSLTarget, string) ([]byte, error) {
			t.Fatal("dispatched without warning")
			return nil, nil
		})
	if code != 1 {
		t.Fatal(code)
	}
}
