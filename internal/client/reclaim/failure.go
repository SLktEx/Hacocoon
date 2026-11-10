package reclaimclient

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

type InvocationError struct{ Failure reclamation.InvocationFailure }

func (e *InvocationError) Error() string {
	return "Windows reclamation " + e.Failure.Phase + " failed at " + e.Failure.Stage
}
func decodeInvocationFailure(data []byte) error {
	if len(data) == 0 || len(data) > 16384 {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	// Decode optional diagnostics separately so an invalid observation cannot
	// discard an otherwise valid primary failure. Envelope/primary fields remain
	// strict; raw optional bytes never leave this decoder.
	var receipt struct {
		Failure struct {
			Phase       string          `json:"phase"`
			Stage       string          `json:"stage"`
			NativeError uint32          `json:"native_error,omitempty"`
			Job         json.RawMessage `json:"job,omitempty"`
		} `json:"failure"`
	}
	if decoder.Decode(&receipt) != nil {
		return nil
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil
	}
	failure := reclamation.InvocationFailure{Phase: receipt.Failure.Phase, Stage: receipt.Failure.Stage, NativeError: receipt.Failure.NativeError}
	if !failure.Valid() {
		return nil
	}
	if len(receipt.Failure.Job) != 0 && failure.Phase == "launch" && failure.Stage == "process_start" {
		jobDecoder := json.NewDecoder(bytes.NewReader(receipt.Failure.Job))
		jobDecoder.DisallowUnknownFields()
		var job reclamation.LaunchJobObservation
		if jobDecoder.Decode(&job) == nil && job.Valid() {
			failure.Job = &job
		}
	}
	return &InvocationError{Failure: failure}
}
