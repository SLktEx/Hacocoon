package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/logging"
)

type nativeReviewFailure struct {
	stage string
	cause error
}

func (e *nativeReviewFailure) Error() string { return "native notification review failed" }
func (e *nativeReviewFailure) Unwrap() error { return e.cause }

type nativeDisplayFailure struct {
	stage  string
	status int64
}

func (e *nativeDisplayFailure) Error() string {
	return fmt.Sprintf("native notification %s failed (HRESULT %d)", e.stage, e.status)
}

// Only fixed local classifications cross this boundary. In particular, private
// controller replies, native output, page tokens and arbitrary errors do not.
func reviewFailureFields(err error) (string, string) {
	stage, reason := "unknown", "unavailable"
	var failure *nativeReviewFailure
	if errors.As(err, &failure) {
		switch failure.stage {
		case "registration", "session_plan", "ownership", "activation", "clear", "peer_start", "review", "events":
			stage = failure.stage
		}
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		reason = "timeout"
	case errors.Is(err, context.Canceled):
		reason = "canceled"
	}
	return stage, reason
}

func reportReviewFailure(out io.Writer, err error) {
	logger, configErr := logging.NewFromEnv(out)
	if configErr != nil {
		// Invalid logging settings must not hide the fixed failure classification.
		logger, _ = logging.New(logging.Config{Writer: out})
	}
	stage, reason := reviewFailureFields(err)
	fields := []any{"component", "client", "operation", "notification_review", "stage", stage, "reason", reason}
	var native *nativeDisplayFailure
	if errors.As(err, &native) {
		switch native.stage {
		case "runtime", "xml", "create", "identity", "show", "history":
			fields = append(fields, "native_stage", native.stage, "native_error", native.status)
		}
	}
	logger.Error("Windows notification review failed", fields...)
}
