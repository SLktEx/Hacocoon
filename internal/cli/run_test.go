package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/env/run"
)

func TestTemporaryProductUsesControllerAndPreservesArgv(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "temporary", true: "retained"}[existing], func(t *testing.T) {
			var got runapp.Spec
			server := control.NewServer()
			if err := server.RegisterStream(controlapi.MethodRun, func(_ context.Context, p json.RawMessage) (control.Stream, error) {
				if err := json.Unmarshal(p, &got); err != nil {
					return nil, err
				}
				return func(_ context.Context, c net.Conn) error {
					return json.NewEncoder(c).Encode(map[string]any{"result": runapp.Result{Environment: "run-example", CleanedUp: true, Execution: runapp.ExecutionResult{ExitCode: 17, Stdout: "output\n", Stderr: "diagnostic\n"}}})
				}, nil
			}); err != nil {
				t.Fatal(err)
			}
			socket := filepath.Join(t.TempDir(), "controller.sock")
			listener, err := control.ListenUnix(socket, 0600)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- server.Serve(ctx, listener) }()
			t.Cleanup(func() { cancel(); <-done })
			t.Setenv("HACO_CONTROL_SOCKET", socket)
			t.Setenv("PATH", t.TempDir())
			args := []string{"--rm"}
			if existing {
				args = append(args, "--workspace", "managed:work", "--read-only", "--no-oci", "--base", "dev")
			}
			args = append(args, "--", "printf", "%s", "space ; $(literal)", "--rm=false")
			var out, diag bytes.Buffer
			if code := temporaryCommand(context.Background(), args, &out, &diag); code != 17 || out.String() != "output\n" || diag.String() != "diagnostic\n" {
				t.Fatalf("code=%d out=%q diag=%q", code, out.String(), diag.String())
			}
			if !reflect.DeepEqual(got.Argv, []string{"printf", "%s", "space ; $(literal)", "--rm=false"}) {
				t.Fatalf("argv=%q", got.Argv)
			}
			if existing {
				if got.WorkspacePath != "managed:work" || got.AccessMode != core.WorkspaceReadOnly || !got.SkipDefaultResource || got.Base != "dev" {
					t.Fatalf("spec=%+v", got)
				}
			} else if got.WorkspacePath != "" || got.SkipDefaultResource {
				t.Fatalf("unexpected default %v", got)
			}
		})
	}
}

func TestTemporaryUsageDoesNotStartWork(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "absent.sock"))
	for _, args := range [][]string{nil, {"--rm=false", "--", "true"}, {"--read-only", "--", "true"}, {"--unknown", "true"}} {
		var out, diag bytes.Buffer
		if code := temporaryCommand(context.Background(), args, &out, &diag); code != 2 {
			t.Fatalf("args=%q code=%d", args, code)
		}
	}
}

func TestTemporaryFailureExplainsBusyWithoutExposingBackend(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		for _, reason := range []string{"busy", "private-unrecognized"} {
			t.Run(language+"/"+reason, func(t *testing.T) {
				t.Setenv("HACO_UI_LANGUAGE", language)
				server := control.NewServer()
				var calls atomic.Int32
				if err := server.RegisterStream(controlapi.MethodRun, func(context.Context, json.RawMessage) (control.Stream, error) {
					return func(_ context.Context, c net.Conn) error {
						calls.Add(1)
						return json.NewEncoder(c).Encode(map[string]any{
							"result": runapp.Result{Environment: "run-refused"},
							"error":  control.NewStatusError(reason, "PRIVATE-BACKEND\nsecret"),
						})
					}, nil
				}); err != nil {
					t.Fatal(err)
				}
				socket := filepath.Join(t.TempDir(), "controller.sock")
				listener, err := control.ListenUnix(socket, 0600)
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				done := make(chan error, 1)
				go func() { done <- server.Serve(ctx, listener) }()
				t.Cleanup(func() { cancel(); <-done })
				t.Setenv("HACO_CONTROL_SOCKET", socket)
				var out, diag bytes.Buffer
				if code := temporaryCommand(ctx, []string{"--json", "--workspace", "managed:work", "--", "true"}, &out, &diag); code != 1 {
					t.Fatalf("code=%d", code)
				}
				var result runapp.Result
				if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.CleanedUp || result.Environment != "run-refused" {
					t.Fatalf("receipt=%s err=%v", out.String(), err)
				}
				wantReason := "failed"
				if reason == "busy" {
					wantReason = "busy"
					want := map[string]string{"en": "Stopping an Environment retains its Workspace lease", "ja": "Envを停止してもWorkspaceの使用権は残ります"}[language]
					if !strings.Contains(diag.String(), want) || !strings.Contains(diag.String(), "haco env list") {
						t.Fatalf("missing busy guidance: %s", diag.String())
					}
				}
				if !strings.Contains(diag.String(), "reason="+wantReason) || !strings.Contains(diag.String(), "run-refused") {
					t.Fatalf("missing reason or cleanup guidance: %s", diag.String())
				}
				if strings.Contains(diag.String(), "PRIVATE") || strings.Contains(diag.String(), "secret") || strings.Contains(diag.String(), "private-unrecognized") {
					t.Fatalf("backend detail exposed: %s", diag.String())
				}
				if calls.Load() != 1 {
					t.Fatalf("run was retried %d times", calls.Load())
				}
			})
		}
	}
}
