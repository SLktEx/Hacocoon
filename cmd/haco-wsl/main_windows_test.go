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

func TestPreparedWorkerDispatch(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "json")
	for _, args := range [][]string{{"_launch"}, {"_continue", "one"}, {"_continue", "one", "two", "extra"}, {"command", "one", "two"}} {
		var out, errout bytes.Buffer
		if code := dispatch(context.Background(), args, &out, &errout, helperActions{}); code != 2 || out.Len() != 0 {
			t.Fatal(code, out.String())
		}
	}
	for _, mode := range []string{"_launch", "_continue"} {
		for _, fail := range []bool{false, true} {
			var out, errout bytes.Buffer
			called := 0
			action := func(ctx context.Context, registration, operation string) error {
				called++
				if registration != "registration" || operation != "operation" {
					t.Fatal("changed argument order")
				}
				if fail {
					return errors.New("token=private-worker-value")
				}
				return nil
			}
			actions := helperActions{worker: action, launch: func(ctx context.Context, r, o string) (int, error) { return 42, action(ctx, r, o) }}
			code := dispatch(context.Background(), []string{mode, "registration", "operation"}, &out, &errout, actions)
			if called != 1 || (code == 1) != fail || strings.Contains(errout.String(), "private-worker-value") {
				t.Fatal(code, called, errout.String())
			}
			if fail {
				if out.Len() != 0 {
					t.Fatal("failure wrote success")
				}
				continue
			}
			if mode == "_launch" && (!strings.Contains(out.String(), "Dispatched Windows worker 42") || strings.Contains(out.String(), "continuation complete")) {
				t.Fatal("dispatch claimed completion", out.String())
			}
			if mode == "_continue" && out.String() != "Prepared WSL continuation complete.\n" {
				t.Fatal(out.String())
			}
		}
	}
}
