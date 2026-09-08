package controlapi

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type snapshotAPIFixture struct {
	saved       core.Snapshot
	err         error
	calls       int
	environment string
}

func (f *snapshotAPIFixture) CaptureSnapshot(_ context.Context, name string) (core.Snapshot, error) {
	f.calls++
	f.environment = name
	return f.saved, f.err
}
func (f *snapshotAPIFixture) ListSnapshots(_ context.Context, name string) ([]core.Snapshot, error) {
	f.calls++
	f.environment = name
	return []core.Snapshot{f.saved}, f.err
}
func (f *snapshotAPIFixture) DeleteSnapshot(context.Context, string) error { f.calls++; return f.err }
func TestSnapshotTransportKeepsOutcomeAndRejectsInvalidRequests(t *testing.T) {
	f := &snapshotAPIFixture{saved: core.Snapshot{ID: "snap-11111111111111111111111111111111", State: "ready", Source: core.SnapshotSource{Environment: core.Environment{Name: "demo", RuntimeRef: "private-runtime"}}, Components: []core.SnapshotComponent{{Role: "rootfs", NativeRef: "private-native", Owner: "private-owner"}, {Role: "workspace:main"}, {Role: "oci"}}}}
	server := control.NewServer()
	if err := RegisterSnapshots(server, f); err != nil {
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
	client, err := NewClient(socket)
	if err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"ready", "recovery-required"} {
		f.saved.State = state
		f.err = core.ErrRecoveryRequired
		response, err := client.Snapshot(ctx, SnapshotRequest{Operation: "create", Environment: "demo"})
		if (err == nil || response.Error == nil || response.Error.Code != "recovery_required") || len(response.Snapshots) != 1 || response.Snapshots[0].State != state || f.environment != "demo" {
			t.Fatal(response, err)
		}
		data, _ := json.Marshal(response)
		if strings.Contains(string(data), "private-") || response.Snapshots[0].Workspaces != 1 || !response.Snapshots[0].OCI {
			t.Fatal(string(data))
		}
	}
	f.err = nil
	for _, req := range []SnapshotRequest{{Operation: "list"}, {Operation: "list", Environment: "demo"}, {Operation: "delete", ID: f.saved.ID}} {
		if _, err := client.Snapshot(ctx, req); err != nil {
			t.Fatal(err)
		}
	}
	before := f.calls
	for _, raw := range []string{`{"operation":"delete","id":"snap-short"}`, `{"operation":"delete","id":"snap-11111111111111111111111111111111","environment":"demo"}`, `{"operation":"create"}`, `{"operation":"list","id":"other"}`, `{"operation":"create","environment":"demo","binding":"private"}`, `{"operation":"restore"}`, `null`} {
		var out SnapshotResponse
		if err := client.wire.Call(ctx, MethodSnapshot, json.RawMessage(raw), &out); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	if f.calls != before {
		t.Fatal("invalid request reached service")
	}
}
