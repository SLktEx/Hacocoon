package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

type forwardReadyWriter struct {
	bytes.Buffer
	cancel context.CancelFunc
}

func (w *forwardReadyWriter) Write(p []byte) (int, error) {
	n, err := w.Buffer.Write(p)
	w.cancel()
	return n, err
}

func TestClientForwardDisplaysUsableAddressInBothLanguages(t *testing.T) {
	// This component covers the native Linux listener. The Windows entry has
	// separate delegation/lifetime tests and installed acceptance.
	t.Setenv("WSL_INTEROP", "")
	t.Setenv("WSL_DISTRO_NAME", "")
	server := control.NewServer()
	if err := server.Register(controlapi.MethodForwardPrepare, func(_ context.Context, payload json.RawMessage) (any, error) {
		var target core.EnvironmentTCPForward
		if err := json.Unmarshal(payload, &target); err != nil {
			return nil, err
		}
		target.Instance = "env-00000000000000000000000000000001"
		return target, nil
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	serverCtx, stop := context.WithCancel(context.Background())
	defer stop()
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(serverCtx, listener) }()
	defer func() { stop(); <-serverDone }()
	t.Setenv("HACO_CONTROL_SOCKET", path)
	for _, language := range []string{"en", "ja"} {
		t.Run(language, func(t *testing.T) {
			t.Setenv("HACO_UI_LANGUAGE", language)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			out := &forwardReadyWriter{cancel: cancel}
			var diagnostic bytes.Buffer
			code := environmentCommand(ctx, []string{"tunnel", "--target-port", "8080", "demo"}, out, &diagnostic)
			if code != 0 || strings.Contains(out.String(), "%!") {
				t.Fatalf("code %d output %q diagnostics %q", code, out.String(), diagnostic.String())
			}
			match := regexp.MustCompile(`127\.0\.0\.1:([0-9]+) → `).FindStringSubmatch(out.String())
			if len(match) != 2 || match[1] == "0" || !strings.Contains(out.String(), "demo") || !strings.Contains(out.String(), "127.0.0.1:8080") || !strings.Contains(out.String(), "Ctrl+C") {
				t.Fatal(out.String())
			}
			// Canceling from the ready writer must close the actual advertised listener.
			conn, err := net.DialTimeout("tcp", "127.0.0.1:"+match[1], 100*time.Millisecond)
			if err == nil {
				_ = conn.Close()
				t.Fatal("listener survived command completion")
			}
		})
	}
}

func TestClientForwardValidatesBeforeControllerAccess(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", "/nonexistent/forward.sock")
	for _, args := range [][]string{{"--target-port", "80", "--listen", "0.0.0.0:80", "demo"}, {"--target-port", "80", "--address", "169.254.254.1", "demo"}, {"--target-port", "80", "--duration", "2h", "demo"}, {"--target-port", "0", "demo"}} {
		var out, diag bytes.Buffer
		if code := forwardClientCommand(context.Background(), args, &out, &diag); code != 2 {
			t.Fatal(args, code, diag.String())
		}
	}
}

func TestClientForwardHelpIsBilingualAndControllerIndependent(t *testing.T) {
	for _, language := range []string{"en", "ja"} {
		t.Setenv("HACO_UI_LANGUAGE", language)
		t.Setenv("HACO_CONTROL_SOCKET", "/nonexistent/forward.sock")
		var out, diag bytes.Buffer
		if code := environmentCommand(context.Background(), []string{"tunnel", "--help"}, &out, &diag); code != 0 {
			t.Fatal(code, diag.String())
		}
		for _, want := range []string{"--target-port", "--listen", "--address", "--duration", "127.0.0.1"} {
			if !strings.Contains(out.String(), want) {
				t.Fatal(language, want, out.String())
			}
		}
	}
}

func TestClientForwardLoggerUsesOnlyDiagnosticWriter(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "json")
	root := logging.Root()
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	for _, want := range []int{0, 1, 2, 17} {
		var out, diagnostic bytes.Buffer
		run := func(ctx context.Context, _ []string, stdout, stderr io.Writer, _ cliui.Language, _ *controlapi.Client, _ func()) int {
			if stdout != &out || stderr != &diagnostic || ctx.Err() != context.Canceled || logging.Root() != root {
				t.Fatal("logging changed streams, cancellation or process logger")
			}
			logging.FromContext(ctx).Error("Windows tunnel companion failed", "component", "client", "operation", "windows_tunnel_companion", "stage", "wait", "reason", "signaled", "context_state", "canceled", "duration_ms", 12)
			return want
		}
		code := forwardClientCommandUsing(parent, []string{"--target-port", "8080", "PRIVATE-env"}, &out, &diagnostic, run)
		var fields map[string]any
		if err := json.Unmarshal(diagnostic.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		if code != want || out.Len() != 0 || fields["operation"] != "windows_tunnel_companion" || fields["reason"] != "signaled" || len(fields) != 9 || strings.Contains(diagnostic.String(), "PRIVATE") {
			t.Fatal("fixed operational fields or command outcome changed")
		}
	}
}

func TestClientForwardRejectsInvalidLoggingBeforeCommand(t *testing.T) {
	t.Setenv("HACO_UI_LANGUAGE", "en")
	t.Setenv("HACO_CONTROL_SOCKET", "/PRIVATE/nonexistent/forward.sock")
	for _, key := range []string{"HACO_LOG_LEVEL", "HACO_LOG_FORMAT"} {
		t.Run(key, func(t *testing.T) {
			t.Setenv("HACO_LOG_LEVEL", "info")
			t.Setenv("HACO_LOG_FORMAT", "text")
			t.Setenv(key, "PRIVATE-invalid")
			var out, diagnostic bytes.Buffer
			run := func(context.Context, []string, io.Writer, io.Writer, cliui.Language, *controlapi.Client, func()) int {
				t.Fatal("invalid logging configuration reached command dispatch")
				return 0
			}
			code := forwardClientCommandUsing(context.Background(), []string{"--target-port", "8080", "PRIVATE-env"}, &out, &diagnostic, run)
			if code != 2 || out.Len() != 0 || diagnostic.String() != cliMessage("error.logging")+"\n" {
				t.Fatalf("code=%d stdout=%q diagnostics=%q", code, out.String(), diagnostic.String())
			}
		})
	}
}

func TestClientForwardLoggingPreservesCommandCodes(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "json")
	t.Setenv("HACO_CONTROL_SOCKET", "/nonexistent/forward.sock")
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"--help"}, 0},
		{[]string{"--target-port", "0", "demo"}, 2},
		{[]string{"--target-port", "8080", "demo"}, 1},
	} {
		var out, diagnostic bytes.Buffer
		if code := forwardClientCommand(context.Background(), tc.args, &out, &diagnostic); code != tc.code {
			t.Fatalf("args=%v code=%d want=%d", tc.args, code, tc.code)
		}
	}
}
