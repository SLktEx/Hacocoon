package reclamation

import "testing"

func TestReportDoesNotConfuseSkippedOrUnmeasuredWithSuccess(t *testing.T) {
	count := uint64(0)
	valid := LinuxReport{Pool: Stage{FilesystemBefore: &FilesystemUsage{CapacityBytes: 100}, FilesystemAfter: &FilesystemUsage{CapacityBytes: 100}, Status: "complete", Attempted: true, Before: &Allocation{LogicalBytes: 100}, After: &Allocation{LogicalBytes: 100}, KernelTrimmedBytes: &count}, Outer: Stage{FilesystemBefore: &FilesystemUsage{CapacityBytes: 100}, FilesystemAfter: &FilesystemUsage{CapacityBytes: 100}, Status: "complete", Attempted: true, KernelTrimmedBytes: &count}}
	if valid.Validate() != nil || !valid.Complete() {
		t.Fatal("zero reclaim is valid")
	}
	for _, mutate := range []func(*LinuxReport){
		func(r *LinuxReport) { r.Pool.Attempted = false },
		func(r *LinuxReport) { r.Pool.KernelTrimmedBytes = nil },
		func(r *LinuxReport) { r.Pool.After = &Allocation{LogicalBytes: 50} },
		func(r *LinuxReport) { r.Outer.Before = &Allocation{LogicalBytes: 100} },
		func(r *LinuxReport) { r.Outer.Status = "skipped" },
		func(r *LinuxReport) { r.Failure = "token=secret" },
	} {
		r := valid
		mutate(&r)
		if r.Validate() == nil {
			t.Fatal("invalid observation accepted", r)
		}
	}
	r := NotStarted("identity_changed")
	if r.Validate() != nil || r.Complete() {
		t.Fatal(r)
	}
}
