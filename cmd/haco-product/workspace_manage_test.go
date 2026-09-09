package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/workspace"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceDeleteReviewsExactIdentityAndHonorsRefusal(t *testing.T) {
	for _, mode := range []string{"yes", "no", "busy", "stale", "json"} {
		t.Run(mode, func(t *testing.T) {
			w := workspace.ManagedWorkspace{Name: "project", State: "ready", Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + strings.Repeat("a", 32)), Path: "managed:project"}, Repositories: []string{"repo"}, Snapshots: []string{"snap-saved"}, Stores: []string{"oci:kept"}}
			if mode == "busy" {
				w.Environments = []string{"dev"}
			}
			deletes := 0
			server := control.NewServer()
			err := server.Register(controlapi.MethodManagedWorkspace, func(_ context.Context, raw json.RawMessage) (any, error) {
				var req controlapi.ManagedWorkspaceRequest
				if err := json.Unmarshal(raw, &req); err != nil {
					t.Fatal(err)
				}
				if req.Operation == "list" {
					return []workspace.ManagedWorkspace{w}, nil
				}
				deletes++
				if req.Workspace != w.Workspace {
					t.Fatal("wrong reviewed identity", req)
				}
				if mode == "stale" {
					return nil, control.NewStatusError("capability_stale", "ownership changed")
				}
				return nil, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			socket := filepath.Join(t.TempDir(), "controller.sock")
			listener, err := control.ListenUnix(socket, 0600)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- server.Serve(ctx, listener) }()
			defer func() { cancel(); <-done }()
			t.Setenv("HACO_CONTROL_SOCKET", socket)
			args := []string{"delete", "project"}
			answer := "yes\n"
			if mode == "no" {
				answer = "n\n"
			}
			if mode == "json" {
				args = []string{"list", "--json"}
			}
			var out, diag bytes.Buffer
			code := managedWorkspaceCommand(ctx, args, strings.NewReader(answer), &out, &diag)
			wantDelete := mode == "yes" || mode == "stale"
			if (deletes == 1) != wantDelete {
				t.Fatal(mode, deletes)
			}
			if (code == 0) != (mode == "yes" || mode == "json") {
				t.Fatal(mode, code, diag.String())
			}
			for _, value := range []string{"workspace:managed:", "repo", "oci:kept", "snap-saved"} {
				if !strings.Contains(out.String(), value) {
					t.Fatal("missing review data", value, out.String())
				}
			}
		})
	}
}
