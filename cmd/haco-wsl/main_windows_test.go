//go:build windows && (amd64 || arm64)

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"syscall"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/wslreclaim"
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
			if mode == "_continue" && out.Len() != 0 {
				t.Fatal(out.String())
			}
		}
	}
}

// Review has its own structured failure boundary; no continuation action runs.
func TestFailedReviewDispatch(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "json")
	for _, args := range [][]string{{"_review-failed"}, {"_review-failed", "r"}, {"_review-failed", "r", "o", "extra"}} {
		var out, errout bytes.Buffer
		if code := dispatch(context.Background(), args, &out, &errout, helperActions{}); code != 2 {
			t.Fatal(code)
		}
	}
	for _, failed := range []bool{false, true} {
		var out, errout bytes.Buffer
		calls := 0
		actions := helperActions{review: func(ctx context.Context, r, o string) error {
			calls++
			if r != "registration" || o != "operation" {
				t.Fatal("changed target")
			}
			if failed {
				return errors.New("token=private-review-value")
			}
			return nil
		}}
		code := dispatch(context.Background(), []string{"_review-failed", "registration", "operation"}, &out, &errout, actions)
		if calls != 1 {
			t.Fatal("review not dispatched exactly once")
		}
		if failed {
			if code != 1 || out.Len() != 0 || strings.Contains(errout.String(), "private-review-value") || !strings.Contains(errout.String(), "review_wsl_failure") {
				t.Fatal("wrong review failure boundary", code, out.String(), errout.String())
			}
		} else if code != 0 || !strings.Contains(out.String(), "Failed result retained") || errout.Len() != 0 {
			t.Fatal("wrong review success", code)
		}
	}
}

func TestStatusWithOrWithoutOperationIsReadOnlyDispatch(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "json")
	for _, invalid := range [][]string{{"_status"}, {"_status", "r", "o", "extra"}} {
		var out, diagnostic bytes.Buffer
		if dispatch(context.Background(), invalid, &out, &diagnostic, helperActions{}) != 2 {
			t.Fatal("invalid status arguments accepted")
		}
	}
	for _, latest := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			var out, diagnostic bytes.Buffer
			calls := 0
			read := func(r string) (wslreclaim.PreparedStatus, error) {
				calls++
				if r != "registration" {
					t.Fatal("changed registration")
				}
				if fail {
					return wslreclaim.PreparedStatus{}, errors.New("token=private-status-value")
				}
				return wslreclaim.PreparedStatus{Operation: "operation", State: "pending"}, nil
			}
			actions := helperActions{
				latest: func(ctx context.Context, r string) (wslreclaim.PreparedStatus, error) {
					if !latest {
						t.Fatal("explicit status used latest")
					}
					return read(r)
				},
				status: func(ctx context.Context, r, o string) (wslreclaim.PreparedStatus, error) {
					if latest || o != "operation" {
						t.Fatal("changed explicit status")
					}
					return read(r)
				},
			}
			args := []string{"_status", "registration"}
			if !latest {
				args = append(args, "operation")
			}
			code := dispatch(context.Background(), args, &out, &diagnostic, actions)
			if calls != 1 || strings.Contains(diagnostic.String(), "private-status-value") {
				t.Fatal("wrong status failure boundary")
			}
			if fail {
				if code != 1 || out.Len() != 0 {
					t.Fatal("failed status claimed a result")
				}
			} else {
				var got wslreclaim.PreparedStatus
				if code != 0 || json.Unmarshal(out.Bytes(), &got) != nil || got.State != "pending" || got.Operation != "operation" || got.Observation != nil {
					t.Fatal("status claimed completion")
				}
			}
		}
	}
}

type rejectedPreparedOutput struct{}

func (rejectedPreparedOutput) Write(p []byte) (int, error) {
	return 0, errors.New("output unavailable")
}

func TestPreparationDoesNotDispatchOrDiscardOnOutputFailure(t *testing.T) {
	t.Setenv("HACO_LOG_LEVEL", "info")
	t.Setenv("HACO_LOG_FORMAT", "json")
	for _, args := range [][]string{{"_prepare"}, {"_prepare", "r", "extra"}} {
		var out, diagnostic bytes.Buffer
		if dispatch(context.Background(), args, &out, &diagnostic, helperActions{}) != 2 {
			t.Fatal("invalid preparation accepted")
		}
	}
	for _, failed := range []bool{false, true} {
		var out, diagnostic bytes.Buffer
		calls := 0
		actions := helperActions{prepare: func(ctx context.Context, r string) (wslreclaim.PreparedStatus, error) {
			calls++
			if r != "registration" {
				t.Fatal("changed registration")
			}
			if failed {
				return wslreclaim.PreparedStatus{}, errors.New("token=private-prepare-value")
			}
			return wslreclaim.PreparedStatus{Operation: "operation", State: "pending"}, nil
		}}
		code := dispatch(context.Background(), []string{"_prepare", "registration"}, &out, &diagnostic, actions)
		if calls != 1 || strings.Contains(diagnostic.String(), "private-prepare-value") {
			t.Fatal("wrong preparation failure boundary")
		}
		if failed {
			if code != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "prepare_wsl_worker") {
				t.Fatal("failed prepare claimed success")
			}
		} else {
			var got wslreclaim.PreparedStatus
			if code != 0 || json.Unmarshal(out.Bytes(), &got) != nil || got.State != "pending" || got.Operation != "operation" || got.Observation != nil {
				t.Fatal("preparation claimed execution")
			}
			diagnostic.Reset()
			if dispatch(context.Background(), []string{"_prepare", "registration"}, rejectedPreparedOutput{}, &diagnostic, actions) != 1 || calls != 2 {
				t.Fatal("output failure retried or succeeded")
			}
			if !strings.Contains(diagnostic.String(), "prepare_wsl_worker") {
				t.Fatal("missing output failure boundary")
			}
		}
	}
}

func TestLaunchFailureLogsNativeCodeWithoutSuccess(t *testing.T) {
	t.Setenv("HACO_LOG_FORMAT", "json")
	var out, diagnostic bytes.Buffer
	code := dispatch(context.Background(), []string{"_launch", "r", "o"}, &out, &diagnostic, helperActions{launch: func(context.Context, string, string) (int, error) { return 0, syscall.Errno(5) }})
	if code != 1 || out.Len() != 0 {
		t.Fatal(code, out.String())
	}
	var record map[string]any
	if err := json.Unmarshal(diagnostic.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record["stage"] != "other" || record["native_error"] != float64(5) {
		t.Fatal(record)
	}
}

func TestInterruptedReviewUsesOnlyExactReviewAction(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var out, diagnostic bytes.Buffer
		calls := 0
		actions := helperActions{interruptedReview: func(ctx context.Context, r, o string) error {
			calls++
			if r != "registration" || o != "operation" {
				t.Fatal("changed identity")
			}
			if fail {
				return errors.New("review refused")
			}
			return nil
		}}
		code := dispatch(context.Background(), []string{"_review-interrupted", "registration", "operation"}, &out, &diagnostic, actions)
		if calls != 1 || (fail && code != 1) || (!fail && code != 0) {
			t.Fatal(code, calls)
		}
		if fail && out.Len() != 0 {
			t.Fatal("failed review claimed success")
		}
		if !fail && !strings.Contains(out.String(), "unknown outcome") {
			t.Fatal(out.String())
		}
	}
}
