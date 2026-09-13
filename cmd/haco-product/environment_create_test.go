package main

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateOCIOptOutUsesExistingCreateRoute(t *testing.T) {
	server := control.NewServer()
	var got controlapi.EnvironmentCreateRequest
	if err := server.Register(controlapi.MethodEnvironmentCreate, func(_ context.Context, payload json.RawMessage) (any, error) {
		err := json.Unmarshal(payload, &got)
		return got, err
	}); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(socket, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() { cancel(); <-done })
	t.Setenv("HACO_CONTROL_SOCKET", socket)
	for _, skip := range []bool{false, true} {
		for _, machine := range []bool{false, true} {
			args := []string{"env", "create", "--workspace", "managed:dev"}
			if skip {
				args = append(args, "--no-oci")
			}
			if machine {
				args = append(args, "--json")
			}
			args = append(args, "dev")
			code, stdout, stderr := captureRun(t, args...)
			if code != 0 || !strings.Contains(stderr, "[succeeded] environment_create") || got.SkipDefaultResource != skip || got.WorkspacePath != "managed:dev" {
				t.Fatalf("code=%d req=%+v err=%s", code, got, stderr)
			}
			if machine {
				var decoded map[string]any
				if json.Unmarshal([]byte(stdout), &decoded) != nil {
					t.Fatal("--json result is not valid JSON", stdout)
				}
			} else {
				if json.Valid([]byte(stdout)) || !strings.Contains(stdout, "name: dev") {
					t.Fatal("default result is not human-readable", stdout)
				}
			}
		}
	}
}
