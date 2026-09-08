//go:build linux

package controlapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"io"
	"testing"
)

type largeAWSDownload struct{ bad bool }

func (*largeAWSDownload) List(context.Context, awsplugin.ListSpec) (core.CapabilityResult, error) {
	return core.CapabilityResult{}, core.ErrUnsupported
}
func (s *largeAWSDownload) Download(_ context.Context, _ awsplugin.GetSpec, w io.Writer) (core.CapabilityResult, error) {
	const chunks = 320
	data := make([]byte, 65536)
	hash := sha256.New()
	for i := 0; i < chunks; i++ {
		hash.Write(data)
		if _, err := w.Write(data); err != nil {
			return core.CapabilityResult{}, err
		}
	}
	receipt := awsplugin.DownloadReceipt{Bytes: chunks * 65536, SHA256: hex.EncodeToString(hash.Sum(nil))}
	if s.bad {
		receipt.SHA256 = "bad"
	}
	output, _ := json.Marshal(receipt)
	return core.CapabilityResult{RequestID: "wire-receipt", Provider: awsplugin.Capability, ExecutionState: core.CapabilitySucceeded, AuditComplete: true, Output: string(output)}, nil
}
func TestAWSDownloadWireHandlesLargeBinaryAndRejectsWrongReceipt(t *testing.T) {
	for _, bad := range []bool{false, true} {
		t.Run(map[bool]string{false: "20MiB", true: "wrong-receipt"}[bad], func(t *testing.T) {
			path := doctorTestSocket(t, func(server *control.Server) {
				if err := RegisterAWS(server, &largeAWSDownload{bad: bad}); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.DownloadS3(context.Background(), awsplugin.GetSpec{Environment: "dev", URL: "s3://example-bucket/data"}, io.Discard)
			if bad {
				if err == nil {
					t.Fatal("bad receipt accepted")
				}
			} else if err != nil || result.RequestID != "wire-receipt" {
				t.Fatal(result, err)
			}
		})
	}
}
