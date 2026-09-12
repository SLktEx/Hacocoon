package diagnostics

import "testing"

func TestPendingMountRequiresVerifiedConfigurationAndNeverSucceeds(t *testing.T) {
	makeReport := func() Report {
		r := Report{}
		for _, name := range CheckNames() {
			r.Checks = append(r.Checks, Check{Name: name, Status: OK, Summary: "Verified"})
		}
		return r
	}
	r := makeReport()
	r.Checks[2] = Check{Name: "storage_mount", Status: Pending, Summary: "Live policy differs", Action: "Arrange Incus-owned maintenance"}
	if r.Validate() != nil || r.Healthy() {
		t.Fatalf("invalid pending semantics: %+v", r)
	}
	r.Checks[1] = Check{Name: "storage", Status: Failed, Summary: "Configuration unavailable", Action: "Inspect Incus"}
	if r.Validate() == nil {
		t.Fatal("pending accepted without configuration")
	}
	r.Checks[2].Status = OK
	r.Checks[2].Action = ""
	if r.Validate() == nil {
		t.Fatal("applied accepted without configuration")
	}
	for i := range CheckNames() {
		if i == 2 {
			continue
		}
		r = makeReport()
		r.Checks[i].Status = Pending
		r.Checks[i].Action = "Wait"
		if r.Validate() == nil {
			t.Fatalf("pending accepted for %s", r.Checks[i].Name)
		}
	}
}
