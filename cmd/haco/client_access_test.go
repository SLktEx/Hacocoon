package main

import (
	"errors"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func TestParsePort(t *testing.T) {
	port, err := parsePort("2222")
	if err != nil || port != 2222 {
		t.Fatalf("port=%d err=%v", port, err)
	}
	for _, raw := range []string{"0", "65536", "nope"} {
		if _, err := parsePort(raw); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatalf("parsePort(%q) err=%v", raw, err)
		}
	}
}
