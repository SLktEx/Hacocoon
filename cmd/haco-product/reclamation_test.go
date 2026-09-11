package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
	"path/filepath"
	"testing"
)

func TestInternalReclaimClientReturnsPartialFailureWithoutRetry(t *testing.T) {
	server := control.NewServer()
	if err := server.Register(controlapi.MethodPing, func(context.Context, json.RawMessage) (any, error) {
		return controlapi.PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
	}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	if err := server.Register(controlapi.MethodReclaimLinux, func(context.Context, json.RawMessage) (any, error) {
		calls++
		return controlapi.ReclaimLinuxResponse{ProtocolVersion: control.ProtocolVersion, LinuxReport: reclamation.NotStarted("identity_changed")}, nil
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
	args := []string{"{11111111-1111-4111-8111-111111111111}", "22222222-2222-4222-8222-222222222222"}
	var out, diagnostic bytes.Buffer
	code := reclaimLinuxClient(ctx, args, &out, &diagnostic)
	var result controlapi.ReclaimLinuxResponse
	if code != 1 || calls != 1 || json.Unmarshal(out.Bytes(), &result) != nil || result.Failure != "identity_changed" || result.Complete() {
		t.Fatal(code, calls, out.String())
	}
}
func TestInternalReclaimClientRejectsSelectionBeforeController(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "absent"))
	for _, args := range [][]string{nil, {"--help"}, {"pool", "path"}, {"{00000000-0000-0000-0000-000000000000}", "22222222-2222-4222-8222-222222222222"}} {
		var out, diagnostic bytes.Buffer
		if reclaimLinuxClient(context.Background(), args, &out, &diagnostic) != 2 || out.Len() != 0 {
			t.Fatal("invalid target reached controller")
		}
	}
}
