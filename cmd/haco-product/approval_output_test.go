package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestApprovalListDefaultsToHumanReadableOutput(t *testing.T) {
	client := &fakeApprovalClient{requests: []core.ApprovalRequest{testApprovalRequest("request-1")}}
	var out, diagnostic bytes.Buffer
	if code := approvalCommand(context.Background(), client, []string{"--list"}, strings.NewReader(""), &out, &diagnostic); code != 0 {
		t.Fatalf("code=%d diagnostic=%s", code, diagnostic.String())
	}
	if json.Valid(out.Bytes()) {
		t.Fatalf("default list output must not be JSON: %q", out.String())
	}
	if !strings.Contains(out.String(), "request_id: request-1") {
		t.Fatalf("human output missing request id: %q", out.String())
	}
}

func TestApprovalListJSONIsExplicit(t *testing.T) {
	client := &fakeApprovalClient{requests: []core.ApprovalRequest{testApprovalRequest("request-1")}}
	var out, diagnostic bytes.Buffer
	if code := approvalCommand(context.Background(), client, []string{"--list", "--json"}, strings.NewReader(""), &out, &diagnostic); code != 0 {
		t.Fatalf("code=%d diagnostic=%s", code, diagnostic.String())
	}
	if !json.Valid(out.Bytes()) || !strings.Contains(out.String(), `"request_id":"request-1"`) {
		t.Fatalf("unexpected JSON output: %q", out.String())
	}
}
