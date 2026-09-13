package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

type productSetupFixture struct{ failure error }

func (f productSetupFixture) SetupHost(ctx context.Context, update recipes.Update) error {
	if update.Script != nil || update.Clear {
		return errors.New("unexpected options")
	}
	return f.failure
}
func productSetupServer(t *testing.T, failure error) string {
	return productSetupServiceServer(t, productSetupFixture{failure})
}
func productSetupServiceServer(t *testing.T, service interface {
	SetupHost(context.Context, recipes.Update) error
}) string {
	t.Helper()
	server := control.NewServer()
	_ = server.Register(controlapi.MethodPing, func(context.Context, json.RawMessage) (any, error) {
		return controlapi.PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
	})
	_ = controlapi.RegisterSetup(server, service)

	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(logging.WithLogger(context.Background(), slog.New(slog.NewTextHandler(io.Discard, nil))))
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	return path
}

func TestProductSetupUsesOnlyController(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", productSetupServer(t, nil))
	t.Setenv("PATH", t.TempDir())
	root := filepath.Join(t.TempDir(), "must-not-exist")
	t.Setenv("HACO_ROOT", root)
	var stdout, stderr bytes.Buffer
	if code := setup(context.Background(), nil, &stdout, &stderr); code != 0 || !strings.Contains(stderr.String(), "[succeeded] setup") {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Host resources prepared") {
		t.Fatalf("output=%s", stdout.String())
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("client created local state: %v", err)
	}
}

func TestProductSetupHelpUsageAndFailures(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "missing.sock"))
	for _, args := range [][]string{{"--help"}, {"--force"}, {"first", "second"}, {"--script", ""}, nil} {
		var stdout, stderr bytes.Buffer
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		code := setup(ctx, args, &stdout, &stderr)
		cancel()
		want := 2
		if len(args) == 0 {
			want = 1
		} else if args[0] == "--help" {
			want = 0
		}
		if code != want {
			t.Fatalf("args=%v code=%d stderr=%s", args, code, stderr.String())
		}
	}
	for _, status := range []string{"busy", "setup_failed", "not_found"} {
		t.Run(status, func(t *testing.T) {
			t.Setenv("HACO_CONTROL_SOCKET", productSetupServer(t, control.NewStatusError(status, "raw-backend-secret")))
			var stdout, stderr bytes.Buffer
			if code := setup(context.Background(), nil, &stdout, &stderr); code != 1 || stdout.Len() != 0 || strings.Contains(stderr.String(), "raw-backend-secret") {
				t.Fatalf("code=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
			}
			if status == "setup_failed" && strings.Contains(stderr.String(), "level=ERROR") {
				t.Fatal("duplicated controller-owned ERROR")
			}
		})
	}
}

func TestProductSetupCanceledBeforeConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "missing.sock"))
	var stdout, stderr bytes.Buffer
	if code := setup(ctx, nil, &stdout, &stderr); code != 1 || stdout.Len() != 0 {
		t.Fatalf("code=%d stdout=%s", code, stdout.String())
	}
}

func TestProjectSetupFailureDiagnosticsAllowOnlyKnownCategories(t *testing.T) {
	stage, code := safeProjectSetupFailure("start", "unavailable")
	if stage != "start" || code != "unavailable" {
		t.Fatal(stage, code)
	}
	stage, code = safeProjectSetupFailure("SECRET-private-script", "SECRET-backend-error")
	if stage != "unknown" || code != "internal" {
		t.Fatal("arbitrary error leaked", stage, code)
	}
}

type readinessSetupClient struct {
	pingErrors []error
	pings      int
}

func (c *readinessSetupClient) Ping(context.Context) (controlapi.PingResponse, error) {
	i := c.pings
	c.pings++
	if i < len(c.pingErrors) {
		return controlapi.PingResponse{}, c.pingErrors[i]
	}
	return controlapi.PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
}
func TestHostSetupWaitsWithoutReplayingMutation(t *testing.T) {
	client := &readinessSetupClient{pingErrors: []error{control.ErrUnavailable}}
	err := waitForSetupController(context.Background(), client)
	if err != nil || client.pings != 2 {
		t.Fatal(err, client.pings)
	}
}
func TestHostSetupReadinessFailureDoesNotMutate(t *testing.T) {
	for _, failure := range []error{control.ErrProtocol, context.Canceled} {
		client := &readinessSetupClient{pingErrors: []error{failure}}
		err := waitForSetupController(context.Background(), client)
		if !errors.Is(err, failure) || client.pings != 1 {
			t.Fatalf("err=%v pings=%d", err, client.pings)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := &readinessSetupClient{}
	if err := waitForSetupController(ctx, client); !errors.Is(err, context.Canceled) || client.pings != 0 {
		t.Fatalf("canceled request contacted controller: %v", err)
	}
}
