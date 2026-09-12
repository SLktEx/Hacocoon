package controlapi

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/snapshotrestore"
)

type restoreAPIFixture struct {
	calls int
	name  string
}

func (f *restoreAPIFixture) RestoreSnapshot(_ context.Context, _ string, name string) (snapshotrestore.Result, error) {
	f.calls++
	f.name = name
	return snapshotrestore.Result{Environment: "dev-restored", Workspace: "restore-owned", State: "cleanup-required"}, core.ErrRecoveryRequired
}
func TestSnapshotRestoreTransportRetainsResidueAndRejectsUnknownFields(t *testing.T) {
	f := &restoreAPIFixture{}
	server := control.NewServer()
	if err := RegisterSnapshotRestore(server, f); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(t.TempDir(), "restore.sock")
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
	response, err := client.RestoreSnapshot(ctx, SnapshotRestoreRequest{ID: "snap-" + strings.Repeat("a", 32), Environment: "new"})
	if err == nil || response.Result.Workspace != "restore-owned" || response.Result.State != "cleanup-required" || f.name != "new" {
		t.Fatal(response, err)
	}
	for _, raw := range []string{`null`, `{"id":"snap-short"}`, `{"id":"snap-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","backup":true}`, `{"id":"snap-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","native_ref":"private"}`} {
		var result any
		if err := client.wire.Call(ctx, MethodSnapshotRestore, json.RawMessage(raw), &result); err == nil {
			t.Fatal("invalid request accepted", raw)
		}
	}
	if f.calls != 1 {
		t.Fatal("invalid request reached implementation", f.calls)
	}
}
