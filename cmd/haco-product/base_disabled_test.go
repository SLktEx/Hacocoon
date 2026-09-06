package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSwitchBaseDisabledBeforeOpeningController(t *testing.T) {
	var out, diagnostic bytes.Buffer
	code := environmentCommand(context.Background(), []string{"switch-base", "--base", "ubuntu", "demo"}, &out, &diagnostic)
	if code != 2 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "currently disabled") || !strings.Contains(diagnostic.String(), "Stage D") {
		t.Fatalf("code=%d out=%s error=%s", code, &out, &diagnostic)
	}
}
