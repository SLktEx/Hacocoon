package reclaimclient

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/SLktEx/Hacocoon/internal/reclamation"
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
	var receipt reclamation.InvocationFailureReceipt
	if decoder.Decode(&receipt) != nil || !receipt.Failure.Valid() {
		return nil
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil
	}
	return &InvocationError{Failure: receipt.Failure}
}
