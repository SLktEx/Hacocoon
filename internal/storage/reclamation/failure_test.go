package reclamation_test

import (
	"encoding/json"
	"testing"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

func TestInvocationDiagnosticsPreserveNativeCodeAndRejectCrossPhaseStages(t *testing.T) {
	for _, tc := range []struct {
		phase, stage string
		valid        bool
	}{
		{"prepare", "disk_access", true},
		{"launch", "readiness", true},
		{"prepare", "other", true},
		{"launch", "other", true},
		{"prepare", "readiness", false},
		{"launch", "disk_access", false},
		{"", "other", false},
		{"complete", "other", false},
		{"prepare", "/private/path", false},
		{"launch", "process_start\nsecret", false},
	} {
		t.Run(tc.phase+"/"+tc.stage, func(t *testing.T) {
			want := reclamation.InvocationFailureReceipt{Failure: reclamation.InvocationFailure{
				Phase: tc.phase, Stage: tc.stage, NativeError: 0xffffffff,
			}}
			data, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var got reclamation.InvocationFailureReceipt
			if err := json.Unmarshal(data, &got); err != nil || got != want {
				t.Fatalf("diagnostic identity changed: got=%+v err=%v", got, err)
			}
			if got.Failure.Valid() != tc.valid {
				t.Fatalf("phase/stage admission=%v, want %v", got.Failure.Valid(), tc.valid)
			}
		})
	}
}
