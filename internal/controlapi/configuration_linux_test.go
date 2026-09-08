//go:build linux

package controlapi

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/control"
)

func TestConfigurationWirePreservesRevisionAndRefusesStaleEdit(t *testing.T) {
	root := t.TempDir()
	service := &capability.PolicyConfiguration{Evaluator: capability.NewFilePolicyEvaluator(filepath.Join(root, "policy.json")), Audit: capability.NewJSONLAudit(filepath.Join(root, "audit", "events.jsonl"))}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterConfiguration(s, service); err != nil {
			t.Fatal(err)
		}
	})
	client, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	first, err := client.ReadConfiguration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	first.Policy = json.RawMessage(`{"default":"deny","rules":[{"capability":"local.echo","action":"echo","resource":"example","environment":"*","decision":"deny"}]}`)
	result, err := client.ReplaceConfiguration(ctx, first)
	if err != nil || result.Revision == "" || result.Revision == first.Revision {
		t.Fatalf("%#v %v", result, err)
	}
	if _, err := client.ReplaceConfiguration(ctx, first); err == nil {
		t.Fatal("wire accepted stale revision")
	}
	got, err := client.ReadConfiguration(ctx)
	if err != nil || got.Revision != result.Revision {
		t.Fatal("wire receipt lost persisted revision")
	}
	if err := client.wire.Call(ctx, MethodConfigurationReplace, map[string]any{"revision": got.Revision, "policy": json.RawMessage(`{"default":"deny","rules":[]}`), "unexpected": true}, nil); err == nil {
		t.Fatal("unknown replacement field accepted")
	}
}
