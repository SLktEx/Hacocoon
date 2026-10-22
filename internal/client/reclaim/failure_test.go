package reclaimclient

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestInvocationFailureOnlyAcceptsBoundedDiagnosticFields(t *testing.T) {
	good := `{"failure":{"phase":"prepare","stage":"disk_access","native_error":5}}`
	var failure *InvocationError
	if !errors.As(decodeInvocationFailure([]byte(good)), &failure) || failure.Failure.Stage != "disk_access" || failure.Failure.NativeError != 5 {
		t.Fatal(failure)
	}
	for _, raw := range []string{
		`{}`, `{"failure":{"phase":"prepare","stage":"/private/path"}}`,
		`{"failure":{"phase":"launch","stage":"disk_access"}}`,
		`{"failure":{"phase":"prepare","stage":"intent","stdout":"token=secret"}}`,
		`{"failure":{"phase":"prepare","stage":"intent","native_error":-1}}`,
		good + good, strings.Repeat("x", 16385),
	} {
		if err := decodeInvocationFailure([]byte(raw)); err != nil {
			t.Fatal("accepted unknown receipt", err)
		}
	}
}

func TestInvocationJobReceiptStrictTypesBoundsAndOldCompatibility(t *testing.T) {
	before := `{"state":"observed","flags":{"breakaway_ok":false,"silent_breakaway_ok":true,"kill_on_job_close":false}}`
	after := `{"state":"unavailable","unavailable_at":"limits","native_error":4294967295}`
	good := `{"failure":{"phase":"launch","stage":"process_start","native_error":5,"job":{"before_start":` + before + `,"after_failure":` + after + `,"outer_jobs":"unknown"}}}`
	var f *InvocationError
	if !errors.As(decodeInvocationFailure([]byte(good)), &f) || f.Failure.Job == nil || f.Failure.NativeError != 5 || f.Failure.Job.AfterFailure.NativeError != 0xffffffff {
		t.Fatal(f)
	}
	old := `{"failure":{"phase":"launch","stage":"process_start","native_error":5}}`
	if !errors.As(decodeInvocationFailure([]byte(old)), &f) || f.Failure.Job != nil {
		t.Fatal("old receipt rejected", f)
	}
	for _, raw := range []string{
		strings.Replace(good, `"outer_jobs":"unknown"`, `"outer_jobs":"observed"`, 1),
		strings.Replace(good, `"outer_jobs":"unknown"`, `"outer_jobs":"unknown","pid":1234`, 1),
		strings.Replace(good, `"outer_jobs":"unknown"`, `"outer_jobs":"unknown","path":"C:\\private"`, 1),
		strings.Replace(good, `"before_start":`+before+`,`, "", 1),
		strings.Replace(good, `"after_failure":`+after+`,`, "", 1),
		strings.Replace(good, `"breakaway_ok":false`, `"breakaway_ok":null`, 1),
		strings.Replace(good, `"breakaway_ok":false`, `"breakaway_ok":0`, 1),
		strings.Replace(good, `"breakaway_ok":false,`, "", 1),
		strings.Replace(good, `"state":"observed"`, `"state":"not_in_job"`, 1),
		strings.Replace(good, `"unavailable_at":"limits"`, `"unavailable_at":"token=secret"`, 1),
		strings.Replace(good, `4294967295`, `4294967296`, 1),
		strings.Replace(good, `4294967295`, `-1`, 1),
		strings.Replace(good, `4294967295`, `1.5`, 1),
		strings.Replace(good, `4294967295`, `true`, 1),
		strings.Replace(good, `"before_start":`+before, `"before_start":null`, 1),
		strings.Replace(good, `"job":{"before_start":`+before+`,"after_failure":`+after+`,"outer_jobs":"unknown"}`, `"job":"private-path"`, 1),
	} {
		var primary *InvocationError
		if !errors.As(decodeInvocationFailure([]byte(raw)), &primary) || primary.Failure.Phase != "launch" || primary.Failure.Stage != "process_start" || primary.Failure.NativeError != 5 || primary.Failure.Job != nil {
			t.Fatal("malformed optional diagnostic replaced primary failure", raw, primary)
		}
	}
}

func TestInvocationJobDoesNotRelaxPrimaryEnvelopeOrReceiptBounds(t *testing.T) {
	good := `{"failure":{"phase":"launch","stage":"process_start","native_error":5,"job":{}}}`
	for _, raw := range []string{
		strings.Replace(good, `"launch"`, `"private"`, 1),
		strings.Replace(good, `"process_start"`, `"private"`, 1),
		strings.Replace(good, `"native_error":5`, `"native_error":4294967296`, 1),
		strings.Replace(good, `"native_error":5`, `"native_error":-1`, 1),
		strings.Replace(good, `"job":{}`, `"job":{},"path":"private"`, 1),
		good + `{}`, good + strings.Repeat(" ", 16384),
	} {
		if decodeInvocationFailure([]byte(raw)) != nil {
			t.Fatal("accepted invalid primary envelope", raw)
		}
	}
	raw := strings.Replace(good, `"process_start"`, `"readiness"`, 1)
	var failure *InvocationError
	if !errors.As(decodeInvocationFailure([]byte(raw)), &failure) || failure.Failure.Stage != "readiness" || failure.Failure.NativeError != 5 || failure.Failure.Job != nil {
		t.Fatal(failure)
	}
}

func TestLegacyClientRejectsNewOptionalObservationWithoutClaimingSuccess(t *testing.T) {
	// The old strict schema does not know job. Mixed old-client/new-helper
	// installations safely fall back to the existing unconfirmed failure, losing
	// categories/code; this patch adds no version negotiation or retry.
	type legacyFailure struct {
		Phase       string `json:"phase"`
		Stage       string `json:"stage"`
		NativeError uint32 `json:"native_error,omitempty"`
	}
	for _, includeJob := range []bool{false, true} {
		raw := `{"failure":{"phase":"launch","stage":"process_start","native_error":5`
		if includeJob {
			raw += `,"job":{"before_start":{"state":"not_in_job"},"after_failure":{"state":"not_in_job"},"outer_jobs":"unknown"}`
		}
		raw += `}}`
		var receipt struct {
			Failure legacyFailure `json:"failure"`
		}
		decoder := json.NewDecoder(bytes.NewReader([]byte(raw)))
		decoder.DisallowUnknownFields()
		err := decoder.Decode(&receipt)
		if (err != nil) != includeJob {
			t.Fatal("legacy schema behavior changed", err)
		}
	}
}
