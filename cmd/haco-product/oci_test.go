package main

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestStoreCreateFromUsesExistingControllerRoute(t *testing.T) {
	server := control.NewServer()
	var got []controlapi.OCIStoreRequest
	if err := server.Register(controlapi.MethodOCIStore, func(_ context.Context, payload json.RawMessage) (any, error) {
		var req controlapi.OCIStoreRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			t.Fatal(err)
		}
		got = append(got, req)
		return controlapi.OCIStoreResponse{Resources: []core.PersistentResource{{ID: req.ID, State: "ready"}}}, nil
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
	t.Setenv("PATH", t.TempDir())
	for _, args := range [][]string{{"plugin", "oci", "store", "create", "dev", "--from", "shared"}, {"plugin", "oci", "store", "create", "--from", "shared", "dev"}} {
		code, out, stderr := captureRun(t, args...)
		if code != 0 || stderr != "" || !strings.Contains(out, `"id":"oci:dev"`) {
			t.Fatalf("code=%d stdout=%s stderr=%s", code, out, stderr)
		}
	}
	expected := controlapi.OCIStoreRequest{Operation: "create", ID: "oci:dev", From: "oci:shared"}
	if !reflect.DeepEqual(got, []controlapi.OCIStoreRequest{expected, expected}) {
		t.Fatalf("requests: %+v", got)
	}
}
func TestStoreFromUsageRejectsAmbiguousOrUnrelatedOperations(t *testing.T) {
	for _, args := range []string{"create dev --from", "create dev --from one --from two", "create dev --force", "delete dev --from shared", "inspect dev --from shared", "list --from shared", "create dev extra", "create --from --delete dev"} {
		if _, ok := parseOCIStoreRequest(append([]string{"oci", "store"}, strings.Fields(args)...)); ok {
			t.Fatalf("accepted %s", args)
		}
	}
}
