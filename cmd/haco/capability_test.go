package main

import (
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParseCapabilityRequest(t *testing.T) {
	req, err := parseCapabilityRequest([]string{"local.echo", "echo", "--resource", "safe", "--environment", "demo", "--param", "message=hello=world"})
	if err != nil {
		t.Fatal(err)
	}
	if req.Capability != "local.echo" || req.Action != "echo" || req.Resource != "safe" || req.Environment != "demo" || req.Parameters["message"] != "hello=world" {
		t.Fatalf("request=%#v", req)
	}
	for _, args := range [][]string{
		{"local.echo"},
		{"local.echo", "echo", "--param", "missing-equals"},
		{"local.echo", "echo", "--param", "a=1", "--param", "a=2"},
		{"local.echo", "echo", "--bogus"},
	} {
		if _, err := parseCapabilityRequest(args); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}
