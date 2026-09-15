package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/api"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/core"
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
