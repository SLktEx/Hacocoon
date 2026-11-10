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

func TestJobObservationStatesAreStrictAndOptional(t *testing.T) {
	truth, falsity := true, false
	flags := &reclamation.JobLimitFlags{BreakawayOK: &falsity, SilentBreakawayOK: &truth, KillOnJobClose: &falsity}
	for _, tc := range []struct {
		name        string
		observation reclamation.JobObservation
		valid       bool
	}{
		{"observed", reclamation.JobObservation{State: "observed", Flags: flags}, true},
		{"not in job", reclamation.JobObservation{State: "not_in_job"}, true},
		{"membership unavailable", reclamation.JobObservation{State: "unavailable", UnavailableAt: "membership", NativeError: 0xffffffff}, true},
		{"limits unavailable", reclamation.JobObservation{State: "unavailable", UnavailableAt: "limits"}, true},
		{"missing flags", reclamation.JobObservation{State: "observed"}, false},
		{"missing boolean", reclamation.JobObservation{State: "observed", Flags: &reclamation.JobLimitFlags{}}, false},
		{"false absent flags", reclamation.JobObservation{State: "not_in_job", Flags: flags}, false},
		{"absent with error", reclamation.JobObservation{State: "not_in_job", NativeError: 5}, false},
		{"unknown false flags", reclamation.JobObservation{State: "unavailable", UnavailableAt: "limits", Flags: flags}, false},
		{"unknown query", reclamation.JobObservation{State: "unavailable", UnavailableAt: "private-path"}, false},
		{"unknown state", reclamation.JobObservation{State: "private-path"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.observation.Valid() != tc.valid {
				t.Fatal(tc.observation)
			}
		})
	}
	o := &reclamation.LaunchJobObservation{BeforeStart: reclamation.JobObservation{State: "not_in_job"}, AfterFailure: reclamation.JobObservation{State: "unavailable", UnavailableAt: "membership"}, OuterJobs: "unknown"}
	f := reclamation.InvocationFailure{Phase: "launch", Stage: "process_start", Job: o}
	if !f.Valid() {
		t.Fatal(f)
	}
	for _, stage := range []string{"pin_executable", "readiness", "other"} {
		f.Stage = stage
		if f.Valid() {
			t.Fatal("unexpected observation stage", stage)
		}
	}
	f.Phase = "prepare"
	f.Stage = "intent"
	if f.Valid() {
		t.Fatal("launch observation on preparation")
	}
	o.OuterJobs = "not_in_job"
	if o.Valid() {
		t.Fatal("outer Jobs claimed observed")
	}
}
