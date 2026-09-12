// Package reclamation defines the bounded results exchanged by the Incus/WSL
// reclamation path. It owns no storage resources, permissions or recovery state.
package reclamation

import (
	"errors"
	"regexp"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type WSLTarget struct {
	RegistrationID string `json:"registration_id"`
	InstallationID string `json:"installation_id"`
}

func (t WSLTarget) Validate() error {
	r := t.RegistrationID
	if len(r) != 38 || r[0] != '{' || r[37] != '}' || !validUUID(r[1:37]) || !validUUID(t.InstallationID) {
		return errors.New("exact managed WSL identity required")
	}
	return nil
}
func validUUID(v string) bool {
	return uuidPattern.MatchString(v) && v != "00000000-0000-0000-0000-000000000000"
}

type Allocation struct {
	LogicalBytes   uint64 `json:"logical_bytes"`
	AllocatedBytes uint64 `json:"allocated_bytes"`
}
type FilesystemUsage struct {
	CapacityBytes uint64 `json:"capacity_bytes"`
	UsedBytes     uint64 `json:"used_bytes"`
}
type Stage struct {
	FilesystemBefore   *FilesystemUsage `json:"filesystem_before,omitempty"`
	FilesystemAfter    *FilesystemUsage `json:"filesystem_after,omitempty"`
	Status             string           `json:"status"`
	Attempted          bool             `json:"attempted"`
	Before             *Allocation      `json:"before,omitempty"`
	After              *Allocation      `json:"after,omitempty"`
	KernelTrimmedBytes *uint64          `json:"kernel_trimmed_bytes,omitempty"`
}
type LinuxReport struct {
	CleanupFailed bool   `json:"cleanup_failed,omitempty"`
	Pool          Stage  `json:"incus_btrfs_loop"`
	Outer         Stage  `json:"wsl_ext4"`
	Failure       string `json:"failure,omitempty"`
}

func NotStarted(reason string) LinuxReport {
	return LinuxReport{Pool: Stage{Status: "skipped"}, Outer: Stage{Status: "skipped"}, Failure: reason}
}
func (r LinuxReport) Complete() bool {
	return !r.CleanupFailed && r.Failure == "" && r.Pool.Status == "complete" && r.Outer.Status == "complete"
}
func (r LinuxReport) Validate() error {
	if (r.CleanupFailed && r.Failure == "") || (r.Failure == "cleanup_failed" && !r.CleanupFailed) {
		return errors.New("invalid cleanup result")
	}
	switch r.Failure {
	case "", "identity_unavailable", "identity_changed", "pool_unavailable", "pool_trim_failed", "outer_trim_failed", "cleanup_failed", "canceled":
	default:
		return errors.New("invalid reclamation failure")
	}
	for i, s := range []Stage{r.Pool, r.Outer} {
		switch s.Status {
		case "complete":
			if s.FilesystemBefore == nil || s.FilesystemAfter == nil || s.FilesystemBefore.CapacityBytes != s.FilesystemAfter.CapacityBytes {
				return errors.New("unproven filesystem capacity")
			}
			if !s.Attempted || s.KernelTrimmedBytes == nil {
				return errors.New("unproven discard completion")
			}
			if i == 0 && (s.Before == nil || s.After == nil || s.Before.LogicalBytes != s.After.LogicalBytes) {
				return errors.New("unproven pool capacity")
			}
		case "failed":
			if r.Failure == "" {
				return errors.New("missing stage failure")
			}
		case "skipped":
			if s.Attempted || s.Before != nil || s.After != nil || s.FilesystemBefore != nil || s.FilesystemAfter != nil || s.KernelTrimmedBytes != nil {
				return errors.New("skipped stage has observations")
			}
		default:
			return errors.New("unknown reclamation stage")
		}
		if !s.Attempted && s.KernelTrimmedBytes != nil {
			return errors.New("unattempted discard count")
		}
		if i == 1 && (s.Before != nil || s.After != nil) {
			return errors.New("ext4 discard is not VHD allocation")
		}
		for _, f := range []*FilesystemUsage{s.FilesystemBefore, s.FilesystemAfter} {
			if f != nil && (f.CapacityBytes == 0 || f.UsedBytes > f.CapacityBytes) {
				return errors.New("invalid filesystem use")
			}
		}
		for _, a := range []*Allocation{s.Before, s.After} {
			if a != nil && a.LogicalBytes == 0 {
				return errors.New("unknown file capacity")
			}
		}
	}
	if r.Failure == "" && !r.Complete() {
		return errors.New("incomplete reclamation has no failure")
	}
	if r.Outer.Attempted && r.Pool.Status != "complete" {
		return errors.New("outer discard after pool failure")
	}
	return nil
}
