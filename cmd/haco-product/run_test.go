package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
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
