package aws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"testing"
)

func TestDownloadRejectsIncompleteOrMismatchedStreams(t *testing.T) {
	for _, mode := range []string{"ok", "empty", "digest", "truncated", "after-receipt", "host-failed", "large-frame"} {
		t.Run(mode, func(t *testing.T) {
			payload := []byte{0, 1, 2, 255}
			if mode == "empty" {
				payload = nil
			}
			hash := sha256.Sum256(payload)
			metadata := DownloadReceipt{Bytes: int64(len(payload)), SHA256: hex.EncodeToString(hash[:])}
			if mode == "digest" {
				metadata.SHA256 = "bad"
			}
			p := &Provider{Stream: func(_ context.Context, _ string, input []byte, out io.Writer) error {
				var req agentRequest
				json.Unmarshal(input, &req)
				if req.Mode != "get" || req.Key != "project/data.bin" || req.Account != testIdentity.Account {
					t.Fatal("wrong target")
				}
				encoder := json.NewEncoder(out)
				if mode == "large-frame" {
					encoder.Encode(downloadFrame{Data: make([]byte, 65537)})
					return nil
				}
				if len(payload) > 0 {
					encoder.Encode(downloadFrame{Data: payload})
				}
				if mode == "truncated" {
					return nil
				}
				encoder.Encode(downloadFrame{Identity: &testIdentity, Receipt: &metadata})
				if mode == "after-receipt" {
					encoder.Encode(downloadFrame{Data: []byte("late")})
				}
				if mode == "host-failed" {
					return errors.New("synthetic secret host error")
				}
				return nil
			}}
			req := getRequest(ListSpec{Environment: "dev", Profile: "default"}, "example-bucket", "project/data.bin", testIdentity)
			result, err := p.Execute(context.WithValue(context.Background(), downloadSinkKey{}, io.Discard), req)
			if mode == "ok" || mode == "empty" {
				if err != nil || result.Output == "" {
					t.Fatal(result, err)
				}
			} else if err == nil {
				t.Fatal("incomplete download accepted")
			}
		})
	}
}
func TestDownloadStreamsBeyondControlEnvelopeWithoutWholeObjectBuffer(t *testing.T) {
	const chunks = 320
	data := make([]byte, 65536)
	for i := range data {
		data[i] = byte(i)
	}
	hash := sha256.New()
	p := &Provider{Stream: func(_ context.Context, _ string, _ []byte, out io.Writer) error {
		encoder := json.NewEncoder(out)
		for i := 0; i < chunks; i++ {
			hash.Write(data)
			if err := encoder.Encode(downloadFrame{Data: data}); err != nil {
				return err
			}
		}
		return encoder.Encode(downloadFrame{Identity: &testIdentity, Receipt: &DownloadReceipt{Bytes: chunks * 65536, SHA256: hex.EncodeToString(hash.Sum(nil))}})
	}}
	result, err := p.Execute(context.WithValue(context.Background(), downloadSinkKey{}, io.Discard), getRequest(ListSpec{Environment: "dev", Profile: "default"}, "example-bucket", "project/data.bin", testIdentity))
	if err != nil {
		t.Fatal(err)
	}
	result.RequestID = "receipt"
	result.ExecutionState = core.CapabilitySucceeded
	result.AuditComplete = true
	if err := VerifyDownload(result, chunks*65536, hex.EncodeToString(hash.Sum(nil))); err != nil {
		t.Fatal(err)
	}
}
func TestDownloadRequiresExplicitSinkAndObjectScope(t *testing.T) {
	p := &Provider{Stream: func(context.Context, string, []byte, io.Writer) error {
		t.Fatal("invalid download executed")
		return nil
	}}
	req := getRequest(ListSpec{Environment: "dev", Profile: "default"}, "example-bucket", "project/data.bin", testIdentity)
	if _, err := p.Execute(context.Background(), req); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal(err)
	}
	ctx := context.WithValue(context.Background(), downloadSinkKey{}, io.Discard)
	for _, key := range []string{"", "../secret", "project/./data", "project/../secret"} {
		req = getRequest(ListSpec{Environment: "dev", Profile: "default"}, "example-bucket", key, testIdentity)
		if _, err := p.Execute(ctx, req); err == nil {
			t.Fatal("ambiguous key accepted", key)
		}
	}
}
