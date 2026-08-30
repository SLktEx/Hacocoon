package controlapi

import "testing"

func TestRejectGuestProjectSelector(t *testing.T) {
	for _, payload := range []string{
		`{"project":"other"}`,
		`{"incus_project":"other"}`,
		`{"incus-project":"other"}`,
	} {
		if err := rejectGuestProjectSelector([]byte(payload)); err == nil {
			t.Fatalf("payload %s unexpectedly accepted", payload)
		}
	}
}

func TestRejectGuestProjectSelectorAllowsOrdinaryWorkloadPayload(t *testing.T) {
	if err := rejectGuestProjectSelector([]byte(`{"environment":"demo","name":"db","image":"oci-docker:library/postgres:18"}`)); err != nil {
		t.Fatalf("ordinary payload rejected: %v", err)
	}
}
