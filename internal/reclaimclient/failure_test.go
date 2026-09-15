package reclaimclient

import (
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
