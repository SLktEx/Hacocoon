package reclamation

import (
	"encoding/json"
	"testing"
)

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

func TestWSLTargetRequiresBothExactNonzeroIdentities(t *testing.T) {
	valid := WSLTarget{RegistrationID: "{12345678-abcd-abcd-abcd-123456789abc}", InstallationID: "abcdefab-1234-1234-1234-123456789abc"}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "00000000-0000-0000-0000-000000000000", "ABCDEFAB-1234-1234-1234-123456789ABC", "../identity", valid.InstallationID + "\n"} {
		for _, field := range []string{"registration", "installation"} {
			target := valid
			if field == "registration" {
				target.RegistrationID = "{" + value + "}"
			} else {
				target.InstallationID = value
			}
			if err := target.Validate(); err == nil {
				t.Fatalf("accepted %s identity %q", field, value)
			}
		}
	}
	for _, value := range []string{valid.InstallationID, "[" + valid.InstallationID + "]", "-" + valid.RegistrationID} {
		target := valid
		target.RegistrationID = value
		if err := target.Validate(); err == nil {
			t.Fatalf("accepted registration shape %q", value)
		}
	}
}

func TestMalformedReclamationReceiptsFailClosedAfterSerialization(t *testing.T) {
	count := uint64(0)
	valid := LinuxReport{Pool: Stage{FilesystemBefore: &FilesystemUsage{CapacityBytes: 100}, FilesystemAfter: &FilesystemUsage{CapacityBytes: 100}, Status: "complete", Attempted: true, Before: &Allocation{LogicalBytes: 100}, After: &Allocation{LogicalBytes: 100}, KernelTrimmedBytes: &count}, Outer: Stage{FilesystemBefore: &FilesystemUsage{CapacityBytes: 100}, FilesystemAfter: &FilesystemUsage{CapacityBytes: 100}, Status: "complete", Attempted: true, KernelTrimmedBytes: &count}}
	for name, mutate := range map[string]func(*LinuxReport){
		"cleanup flag without reason":  func(r *LinuxReport) { r.CleanupFailed = true },
		"cleanup reason without flag":  func(r *LinuxReport) { r.Failure = "cleanup_failed" },
		"missing capacity":             func(r *LinuxReport) { r.Pool.FilesystemBefore = nil },
		"changed capacity":             func(r *LinuxReport) { r.Outer.FilesystemAfter = &FilesystemUsage{CapacityBytes: 99} },
		"missing pool size":            func(r *LinuxReport) { r.Pool.Before = nil },
		"stage failure without reason": func(r *LinuxReport) { r.Pool.Status = "failed" },
		"unknown stage":                func(r *LinuxReport) { r.Pool.Status = "success" },
		"unattempted count": func(r *LinuxReport) {
			r.Pool.Status = "failed"
			r.Pool.Attempted = false
			r.Failure = "pool_trim_failed"
		},
		"impossible usage": func(r *LinuxReport) { r.Pool.FilesystemAfter = &FilesystemUsage{CapacityBytes: 100, UsedBytes: 101} },
		"zero capacity": func(r *LinuxReport) {
			r.Pool.FilesystemAfter = &FilesystemUsage{}
			r.Pool.FilesystemBefore = &FilesystemUsage{}
		},
		"zero file size":       func(r *LinuxReport) { r.Pool.Before = &Allocation{}; r.Pool.After = &Allocation{} },
		"skip without failure": func(r *LinuxReport) { *r = NotStarted("") },
		"outer after failed pool": func(r *LinuxReport) {
			r.Pool = Stage{Status: "failed", Attempted: true}
			r.Failure = "pool_trim_failed"
		},
	} {
		t.Run(name, func(t *testing.T) {
			r := valid
			mutate(&r)
			data, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var received LinuxReport
			if err := json.Unmarshal(data, &received); err != nil {
				t.Fatal(err)
			}
			if received.Validate() == nil {
				t.Fatalf("accepted malformed receipt: %s", data)
			}
		})
	}
}
