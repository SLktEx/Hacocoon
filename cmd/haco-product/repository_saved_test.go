package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/core"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductGitSavedDecisionUsesControllerReceipt(t *testing.T) {
	for _, tc := range []struct {
		action, save, want string
		approved           bool
	}{
		{"approve", "env", "allow-environment", true},
		{"deny", "env", "deny-environment", false},
		{"approve", "all", "allow-global", true},
		{"deny", "all", "deny-global", false},
		{"approve", "ask-env", "ask-environment", true},
		{"deny", "ask-all", "ask-global", false},
	} {
		t.Run(tc.action+"-"+tc.save, func(t *testing.T) {
			server := control.NewServer()
			_ = server.Register(controlapi.MethodGitPending, func(context.Context, json.RawMessage) (any, error) {
				return []map[string]any{{"id": "pending-id", "saved_scope": core.CapabilityRequest{Capability: "git.repository", Action: "push", Resource: "target"}}}, nil
			})
			_ = server.Register(controlapi.MethodGitDecide, func(_ context.Context, raw json.RawMessage) (any, error) {
				var req controlapi.GitDecisionRequest
				if err := json.Unmarshal(raw, &req); err != nil {
					t.Error(err)
				}
				if req.ID != "pending-id" || req.Approved != tc.approved || string(req.Save) != tc.want {
					t.Errorf("wrong decision: %+v", req)
				}
				return core.CapabilityResult{SavedChoice: string(req.Save), ExecutionState: core.CapabilityNotExecuted}, nil
			})
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
			var out, diag bytes.Buffer
			if code := repositoryCommand(ctx, "git", []string{tc.action, "--save", tc.save, "pending-id"}, &out, &diag); code != 0 {
				t.Fatalf("code=%d %s", code, &diag)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Fatal("missing controller save receipt")
			}
		})
	}
}
func TestProductGitInvalidSavedScopeRejectedBeforeController(t *testing.T) {
	t.Setenv("HACO_CONTROL_SOCKET", filepath.Join(t.TempDir(), "missing"))
	var out, diag bytes.Buffer
	if code := repositoryCommand(context.Background(), "git", []string{"approve", "--save", "internet", "id"}, &out, &diag); code != 2 || out.Len() != 0 {
		t.Fatalf("invalid scope accepted: %d", code)
	}
}
