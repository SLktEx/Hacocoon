package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
)

func productSetupServer(t *testing.T, failure error) string {
	t.Helper()
	server := control.NewServer()
	_ = server.Register(controlapi.MethodSetup, func(_ context.Context, payload json.RawMessage) (any, error) {
		if len(payload) != 0 {
			t.Errorf("setup accepted caller parameters: %q", payload)
		}
		return controlapi.PingResponse{ProtocolVersion: control.ProtocolVersion}, failure
	})
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
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
	if code := setup(context.Background(), nil, &stdout, &stderr); code != 0 || stderr.Len() != 0 {
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
	for _, args := range [][]string{{"--help"}, {"--force"}, {"path"}, nil} {
		var stdout, stderr bytes.Buffer
		code := setup(context.Background(), args, &stdout, &stderr)
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
