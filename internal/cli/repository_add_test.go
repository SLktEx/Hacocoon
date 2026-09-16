package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"sync/atomic"
	"testing"

	controlapi "github.com/SLktEx/Hacocoon/internal/controller/api"
	control "github.com/SLktEx/Hacocoon/internal/controller/transport"
)

func TestRepoAddSendsOnlyRepositoryIdentity(t *testing.T) {
	server := control.NewServer()
	var calls atomic.Int32
	if err := server.Register(controlapi.MethodRepositoryAdd, func(_ context.Context, raw json.RawMessage) (any, error) {
		calls.Add(1)
		var fields map[string]string
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Error(err)
			return nil, err
		}
		if len(fields) != 2 || fields["id"] != "sample" || fields["remote"] != "https://github.com/OWNER/REPO.git" {
			t.Errorf("registration has non-identity fields: %s", raw)
		}
		return fields, nil
	}); err != nil {
		t.Fatal(err)
	}
	var workspaceCalls atomic.Int32
	if err := server.Register(controlapi.MethodWorkspaceCopy, func(_ context.Context, raw json.RawMessage) (any, error) {
		workspaceCalls.Add(1)
		var req controlapi.WorkspaceCopyRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Error(err)
			return nil, err
		}
		if req.ID != "work" || req.Repository != "sample" || req.Branch != "feature/foo" {
			t.Errorf("checkout selection lost: %+v", req)
		}
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "control.sock")
	listener, err := control.ListenUnix(path, 0600)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	defer func() { cancel(); <-done }()
	t.Setenv("HACO_CONTROL_SOCKET", path)
	var out, diagnostic bytes.Buffer
	if code := repositoryCommand(ctx, "repo", []string{"add", "sample", "https://github.com/OWNER/REPO.git"}, &out, &diagnostic); code != 0 || calls.Load() != 1 {
		t.Fatalf("code=%d calls=%d %s", code, calls.Load(), &diagnostic)
	}
	for _, args := range [][]string{
		{"add", "--branch", "main", "sample", "https://github.com/OWNER/REPO.git"},
		{"clone", "--branch", "main", "sample", "https://github.com/OWNER/REPO.git"},
		{"clone", "sample", "https://github.com/OWNER/REPO.git"},
	} {
		if code := repositoryCommand(ctx, "repo", args, &out, &diagnostic); code != 2 || calls.Load() != 1 {
			t.Fatal("retired interface reached registration", args, code, calls.Load())
		}
	}

	if code := repositoryCommand(ctx, "workspace", []string{"create", "--repo", "sample", "--branch", "feature/foo", "work"}, &out, &diagnostic); code != 0 || workspaceCalls.Load() != 1 {
		t.Fatal("Workspace selection failed", code, &diagnostic)
	}
	if code := repositoryCommand(ctx, "workspace", []string{"create", "--repo", "sample,other", "--branch", "feature/foo", "work"}, &out, &diagnostic); code != 2 || workspaceCalls.Load() != 1 {
		t.Fatal("ambiguous collection branch reached controller", code)
	}
}
