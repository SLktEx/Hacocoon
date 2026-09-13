package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestReviewFailureLogsOnlyFixedClassifications(t *testing.T) {
	t.Setenv("HACO_LOG_FORMAT", "json")
	t.Setenv("HACO_LOG_LEVEL", "debug")
	for _, tc := range []struct {
		stage                 string
		cause                 error
		wantStage, wantReason string
	}{
		{"review", errors.New("private-page-token raw-controller-output"), "review", "unavailable"},
		{"clear", context.DeadlineExceeded, "clear", "timeout"},
		{"peer_start", context.Canceled, "peer_start", "canceled"},
		{"private-stage-secret", errors.New("private-error-secret"), "unknown", "unavailable"},
	} {
		var output bytes.Buffer
		reportReviewFailure(&output, &nativeReviewFailure{stage: tc.stage, cause: tc.cause})
		if strings.Contains(output.String(), "private") || strings.Contains(output.String(), "raw-controller") {
			t.Fatal("private content reached log")
		}
		var entry map[string]any
		if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
			t.Fatal(err)
		}
		if entry["stage"] != tc.wantStage || entry["reason"] != tc.wantReason || entry["operation"] != "notification_review" || entry["level"] != "ERROR" {
			t.Fatalf("invalid fixed diagnostics: %+v", entry)
		}
	}
}

func TestInvalidLogSettingsStillReportSafeFailure(t *testing.T) {
	t.Setenv("HACO_LOG_FORMAT", "private-setting")
	var output bytes.Buffer
	reportReviewFailure(&output, errors.New("private-error"))
	if strings.Contains(output.String(), "private") || !strings.Contains(output.String(), "stage=unknown reason=unavailable") {
		t.Fatal(output.String())
	}
}

func TestNativeStatusFieldsAreTypedAndBounded(t *testing.T) {
	t.Setenv("HACO_LOG_FORMAT", "json")
	var output bytes.Buffer
	reportReviewFailure(&output, &nativeReviewFailure{stage: "clear", cause: &nativeDisplayFailure{stage: "history", status: -2146233087}})
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatal(err)
	}
	if entry["native_stage"] != "history" || entry["native_error"] != float64(-2146233087) {
		t.Fatal(entry)
	}
	output.Reset()
	reportReviewFailure(&output, &nativeDisplayFailure{stage: "private-token", status: 1})
	if strings.Contains(output.String(), "private") || strings.Contains(output.String(), "native_error") {
		t.Fatal("arbitrary native stage logged")
	}
}
