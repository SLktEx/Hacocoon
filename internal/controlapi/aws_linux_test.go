//go:build linux

package controlapi

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	awsplugin "github.com/SLktEx/Hacocoon/modules/capability/aws"
	"testing"
)

type fakeAWS struct{ calls int }

func (f *fakeAWS) List(_ context.Context, s awsplugin.ListSpec) (core.CapabilityResult, error) {
	f.calls++
	return core.CapabilityResult{RequestID: "fixed", ExecutionState: core.CapabilityFailed, AuditComplete: true}, awsplugin.ErrAWSRejected
}
func TestAWSWirePreservesFailedExecutionAndRejectsExtraAuthority(t *testing.T) {
	service := &fakeAWS{}
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterAWS(s, service); err != nil {
			t.Fatal(err)
		}
	})
	c, err := NewClient(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.ListS3(context.Background(), awsplugin.ListSpec{Environment: "dev", URL: "s3://example-bucket"})
	if err == nil || result.RequestID != "fixed" || result.ExecutionState != core.CapabilityFailed || !result.AuditComplete {
		t.Fatal("lost failed receipt", result, err)
	}
	if err := c.wire.Call(context.Background(), MethodAWSList, map[string]any{"environment": "dev", "url": "s3://example-bucket", "endpoint": "https://evil.invalid"}, nil); err == nil || service.calls != 1 {
		t.Fatal("extra authority accepted")
	}
	var status *control.StatusError
	if !errors.As(err, &status) {
		t.Fatal("missing structured error")
	}
}
