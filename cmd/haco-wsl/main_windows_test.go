//go:build windows && (amd64 || arm64)

package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
)

func TestInstallerHelperArgumentsAndFailureBoundary(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "json")
	for _, args := range [][]string{nil, {"enroll"}, {"compact", "id"}, {"enroll", "id", "extra"}} {
		var out, errout bytes.Buffer
		code := run(context.Background(), args, &out, &errout, func(context.Context, string) error { t.Fatal("unexpected enrollment"); return nil })
		if code != 2 || out.Len() != 0 {
			t.Fatal(code, out.String())
		}
	}
	var out, errout bytes.Buffer
	code := run(context.Background(), []string{"enroll", "{11111111-1111-4111-8111-111111111111}"}, &out, &errout, func(ctx context.Context, id string) error { return errors.New("token=private-value") })
	if code != 1 || out.Len() != 0 || strings.Contains(errout.String(), "private-value") || !strings.Contains(errout.String(), "enroll_wsl") {
		t.Fatal(code, out.String(), errout.String())
	}
	out.Reset()
	errout.Reset()
	code = run(context.Background(), []string{"enroll", "id"}, &out, &errout, func(context.Context, string) error { return nil })
	if code != 0 || out.String() != "Managed WSL enrollment complete.\n" || errout.Len() != 0 {
		t.Fatal(code, out.String(), errout.String())
	}
}
