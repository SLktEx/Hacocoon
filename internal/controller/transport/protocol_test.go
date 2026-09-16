package control

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorEnvelopePreservesInvalidArgumentsAndExplicitStatus(t *testing.T) {
	for _, tc := range []struct {
		name          string
		err           error
		code, message string
	}{
		{"invalid", ErrInvalidArgument, "invalid_argument", ErrInvalidArgument.Error()},
		{"wrapped-invalid", fmt.Errorf("network request: %w", ErrInvalidArgument), "invalid_argument", "network request: invalid control argument"},
		{"explicit", errors.Join(NewStatusError("recovery_required", "retained ownership"), ErrInvalidArgument), "recovery_required", "retained ownership"},
		{"internal", errors.New("operation failed"), "internal", "operation failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := errorEnvelope(tc.err)
			err := validateResponse(response)
			var status *StatusError
			if response.Version != ProtocolVersion || !errors.As(err, &status) || status.Code != tc.code || status.Message != tc.message {
				t.Fatal("error boundary changed failure category", response, err)
			}
		})
	}
}
