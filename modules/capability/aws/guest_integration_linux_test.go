//go:build linux

package aws

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/approvals"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGuestHTTPUsesOrdinaryApprovalAndSavedRevocation(t *testing.T) {
	root := t.TempDir()
	policyPath := filepath.Join(root, "policy.json")
	if err := os.WriteFile(policyPath, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	queue := approvals.New()
	calls := 0
	host := fixtureHost(t, &calls)
	service, err := capability.New(capability.NewFilePolicyEvaluator(policyPath), queue, capability.NewJSONLAudit(filepath.Join(root, "audit.jsonl")), &Provider{Host: host})
	if err != nil {
		t.Fatal(err)
	}
	service.ConfigureEnvironmentIdentity(environments{})
	broker := &Broker{Host: host, Environments: environments{}, Capabilities: service}
	handler := NewGuestHandler(broker, guestSourceFunc(func(context.Context, net.IP) (string, string, error) {
		return "dev", "env-11111111111111111111111111111111", nil
	}))
	// Synthetic source evidence, real HTTP socket and shared approval/Policy/audit.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.RemoteAddr = "10.200.0.2:1234"
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	client := NewGuestClient()
	client.endpoint = server.URL + GuestPath
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	spec := ListSpec{URL: "s3://example-bucket/project/"}
	done := make(chan error, 1)
	go func() { _, err := client.ListS3(ctx, spec); done <- err }()
	var pending []core.ApprovalRequest
	for len(pending) == 0 && ctx.Err() == nil {
		pending = queue.PendingApprovals()
		if len(pending) == 0 {
			time.Sleep(time.Millisecond)
		}
	}
	if len(pending) != 1 {
		t.Fatal("guest request not pending")
	}
	prompt := pending[0]
	if prompt.CapabilityRequest.Environment != "dev" || prompt.CapabilityRequest.EnvironmentInstance == "" {
		t.Fatal("source missing")
	}
	receipt, err := queue.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true, Save: capability.AllowEnvironment})
	if err != nil || !receipt.AuditComplete {
		t.Fatal(receipt, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListS3(ctx, spec); err != nil {
		t.Fatal("saved permission not reused", err)
	}
	if err := os.WriteFile(policyPath, []byte(`{"default":"deny","rules":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ListS3(ctx, spec); err == nil {
		t.Fatal("revocation ignored")
	}
}
