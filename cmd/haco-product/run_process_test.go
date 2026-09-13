package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

func TestTemporaryProductStreamsPipeAndPreservesExit(t *testing.T) {
	server := control.NewServer()
	var got controlapi.RunStreamRequest
	if err := server.RegisterStream(controlapi.MethodRunStream, func(_ context.Context, p json.RawMessage) (control.Stream, error) {
		if err := json.Unmarshal(p, &got); err != nil {
			return nil, err
		}
		return func(ctx context.Context, conn net.Conn) error {
			return control.ServeProcess(ctx, conn, func(_ context.Context, in io.Reader, out, diagnostic io.Writer) ([]byte, error) {
				if _, err := io.Copy(out, in); err != nil {
					return nil, err
				}
				if _, err := io.WriteString(diagnostic, "process stderr"); err != nil {
					return nil, err
				}
				return json.Marshal(map[string]any{"result": runapp.Result{Environment: "run-piped", CleanedUp: true, Execution: runapp.ExecutionResult{ExitCode: 17}}, "error": map[string]any{"code": "internal", "message": "process exit", "exit_code": 17}})
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
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	setCLITestLocale(t, "C")
	data := bytes.Repeat([]byte("piped input\n"), 100000)
	var out, diagnostic bytes.Buffer
	args := []string{"-i", "--workspace", "managed:retained", "--", "cat", "--literal ; $(no-execution)"}
	code := temporaryCommandWithInput(ctx, args, bytes.NewReader(data), &out, &diagnostic)
	if code != 17 || !bytes.Equal(out.Bytes(), data) || diagnostic.String() != "process stderr" {
		t.Fatal("CLI changed stream or exit", code, diagnostic.String())
	}
	if got.TTY || got.Spec.WorkspacePath != "managed:retained" || !reflect.DeepEqual(got.Spec.Argv, args[4:]) {
		t.Fatal("CLI changed request", got)
	}
}

func TestTemporaryProductRejectsPipeTTYAndJSONBeforeController(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "absent.sock"))
	setCLITestLocale(t, "ja_JP.UTF-8")
	for _, args := range [][]string{{"-it", "--", "bash"}, {"-i", "--json", "--", "cat"}} {
		var out, diagnostic bytes.Buffer
		if code := temporaryCommandWithInput(context.Background(), args, strings.NewReader("input"), &out, &diagnostic); code != 2 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "haco:") || strings.Contains(diagnostic.String(), "requires") {
			t.Fatal(code, diagnostic.String())
		}
	}
}
