//go:build linux

package controlapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/review"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"github.com/SLktEx/Hacocoon/modules/standard/approvals"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type awsEnvironments struct{}

func (awsEnvironments) GetEnvironment(_ context.Context, n string) (core.Environment, error) {
	return core.Environment{Name: n}, nil
}
func (awsEnvironments) EnvironmentInstance(context.Context, core.Environment) (string, error) {
	return "env-11111111111111111111111111111111", nil
}
func TestAWSRequestUsesOrdinaryPendingReviewAndSavedPolicy(t *testing.T) {
	for _, operation := range []string{"list", "download"} {
		t.Run(operation, func(t *testing.T) {
			root := t.TempDir()
			policyPath := filepath.Join(root, "policy.json")
			if err := os.WriteFile(policyPath, []byte(`{"default":"require-approval","rules":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			queue := approvals.New()
			var calls atomic.Int32
			h := awsplugin.Host(func(_ context.Context, _ string, input []byte) ([]byte, error) {
				var req map[string]string
				if err := json.Unmarshal(input, &req); err != nil {
					return nil, err
				}
				result := map[string]any{"identity": map[string]string{"account": "123456789012", "principal": "arn:aws:sts::123456789012:assumed-role/Developer/session", "region": "ap-northeast-1"}}
				if req["mode"] == "list" {
					calls.Add(1)
					result["objects"] = []map[string]any{{"key": "project/config.json", "size": 42}}
				}
				return json.Marshal(result)
			})
			svc, err := capability.New(capability.NewFilePolicyEvaluator(policyPath), queue, capability.NewJSONLAudit(filepath.Join(root, "audit", "events.jsonl")), &awsplugin.Provider{Host: h, Stream: func(_ context.Context, _ string, _ []byte, w io.Writer) error {
				calls.Add(1)
				data := []byte{0, 1, 2, 255}
				digest := sha256.Sum256(data)
				encoder := json.NewEncoder(w)
				if err := encoder.Encode(map[string]any{"data": data}); err != nil {
					return err
				}
				return encoder.Encode(map[string]any{
					"identity": map[string]string{"account": "123456789012", "principal": "arn:aws:sts::123456789012:assumed-role/Developer/session", "region": "ap-northeast-1"},
					"receipt":  awsplugin.DownloadReceipt{Bytes: int64(len(data)), SHA256: hex.EncodeToString(digest[:])},
				})
			}})
			if err != nil {
				t.Fatal(err)
			}
			svc.ConfigureEnvironmentIdentity(awsEnvironments{})
			broker := &awsplugin.Broker{Host: h, Capabilities: svc, Environments: awsEnvironments{}}
			path := doctorTestSocket(t, func(server *control.Server) {
				if err := RegisterAWS(server, broker); err != nil {
					t.Fatal(err)
				}
				if err := RegisterReviews(server, review.New(queue)); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			type outcome struct {
				result core.CapabilityResult
				err    error
			}
			done := make(chan outcome, 1)
			spec := awsplugin.ListSpec{Environment: "dev", URL: "s3://example-bucket/project/"}
			if operation == "download" {
				spec.URL = "s3://example-bucket/project/data.bin"
			}
			perform := func() (core.CapabilityResult, error) {
				if operation == "download" {
					return client.DownloadS3(ctx, awsplugin.GetSpec(spec), io.Discard)
				}
				return client.ListS3(ctx, spec)
			}
			go func() { r, e := perform(); done <- outcome{r, e} }()
			var pending []core.ApprovalRequest
			for len(pending) == 0 && ctx.Err() == nil {
				pending, err = client.PendingApprovals(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if len(pending) == 0 {
					time.Sleep(10 * time.Millisecond)
				}
			}
			if len(pending) != 1 || calls.Load() != 0 {
				t.Fatal("request bypassed review", pending)
			}
			prompt := pending[0]
			if prompt.CapabilityRequest.Attributes["account"] != "123456789012" || prompt.CapabilityRequest.Action != map[string]string{"list": "ListObjectsV2", "download": "GetObject"}[operation] || prompt.SavedScope == nil {
				t.Fatal("review missing AWS scope")
			}
			receipt, err := client.DecideApproval(ctx, prompt.RequestID, capability.ApprovalDecision{Approved: true, Save: capability.AllowEnvironment})
			if err != nil || receipt.ExecutionState != core.CapabilitySucceeded || receipt.SavedChoice != string(capability.AllowEnvironment) {
				t.Fatal(receipt, err)
			}
			finished := <-done
			if finished.err != nil || finished.result.RequestID != receipt.RequestID || calls.Load() != 1 {
				t.Fatal("decision result mismatch", finished)
			}
			if _, err := perform(); err != nil || calls.Load() != 2 {
				t.Fatal("saved scope not reused", err)
			}
			if err := os.WriteFile(policyPath, []byte(`{"default":"deny","rules":[]}`), 0600); err != nil {
				t.Fatal(err)
			}
			result, err := perform()
			if err == nil || result.ExecutionState != core.CapabilityNotExecuted || calls.Load() != 2 {
				t.Fatal("revoked request executed", result, err)
			}
		})
	}
}
