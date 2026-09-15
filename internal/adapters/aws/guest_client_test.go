package aws

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuestClientVerifiesTransferCompletion(t *testing.T) {
	for _, mode := range []string{"list", "get", "truncated", "digest", "after-receipt", "denied", "redirect", "caller-env"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var request map[string]string
				if json.NewDecoder(r.Body).Decode(&request) != nil || request["environment"] != "" || request["instance"] != "" {
					t.Error("caller authority sent")
				}
				if mode == "redirect" {
					w.Header().Set("Location", "http://127.0.0.1:1/")
					w.WriteHeader(302)
					return
				}
				w.Header().Set("Content-Type", "application/x-ndjson")
				encoder := json.NewEncoder(w)
				result := core.CapabilityResult{RequestID: "request-fixture", Provider: Capability, ExecutionState: core.CapabilitySucceeded, AuditComplete: true, Output: "[]"}
				if request["operation"] == "get" {
					data := []byte{0, 1, 2, 255}
					digest := sha256.Sum256(data)
					encoder.Encode(GuestFrame{Data: data})
					receipt := DownloadReceipt{Bytes: 4, SHA256: hex.EncodeToString(digest[:])}
					if mode == "digest" {
						receipt.SHA256 = "wrong"
					}
					encoded, _ := json.Marshal(receipt)
					result.Output = string(encoded)
				}
				if mode == "truncated" {
					return
				}
				failure := ""
				if mode == "denied" {
					failure = "untrusted raw diagnostic"
				}
				encoder.Encode(GuestFrame{Result: &result, Error: failure})
				if mode == "after-receipt" {
					encoder.Encode(GuestFrame{Data: []byte("late")})
				}
			}))
			defer server.Close()
			client := NewGuestClient()
			client.endpoint = server.URL
			spec := ListSpec{URL: "s3://example-bucket/project/data"}
			if mode == "caller-env" {
				spec.Environment = "other"
			}
			var result core.CapabilityResult
			var err error
			if mode == "list" {
				result, err = client.ListS3(context.Background(), spec)
			} else {
				result, err = client.DownloadS3(context.Background(), GetSpec(spec), io.Discard)
			}
			if mode == "list" || mode == "get" {
				if err != nil || !result.AuditComplete {
					t.Fatal(result, err)
				}
			} else if err == nil {
				t.Fatal("incomplete request accepted")
			}
			if err != nil && strings.Contains(err.Error(), "untrusted raw") {
				t.Fatal("raw error exposed")
			}
			if mode == "caller-env" && calls != 0 {
				t.Fatal("caller-selected identity sent")
			}
			if mode == "redirect" && calls != 1 {
				t.Fatal("redirect followed")
			}
		})
	}
}
