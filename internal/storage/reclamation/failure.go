package reclamation

// InvocationFailure contains only stable diagnostic categories. It is never a
// receipt of absence, authorization to retry, or provider/subprocess output.
type InvocationFailure struct {
	Phase       string                `json:"phase"`
	Stage       string                `json:"stage"`
	NativeError uint32                `json:"native_error,omitempty"`
	Job         *LaunchJobObservation `json:"job,omitempty"`
}

func (f InvocationFailure) Valid() bool {
	if f.Job != nil && (f.Phase != "launch" || f.Stage != "process_start" || !f.Job.Valid()) {
		return false
	}
	if f.Phase == "prepare" {
		switch f.Stage {
		case "registration", "exclusion", "disk_path", "disk_access", "record_access", "windows_owner", "enrollment", "installation", "binding", "intent", "other":
			return true
		}
	}
	if f.Phase == "launch" {
		switch f.Stage {
		case "pin_executable", "process_start", "readiness", "other":
			return true
		}
	}
	return false
}

type InvocationFailureReceipt struct {
	Failure InvocationFailure `json:"failure"`
}

// LaunchJobObservation describes only the calling launcher's immediate Job at
// Cmd.Start, not the child, its ancestry, or the cause of a launch failure.
// Outer Jobs remain unknown even when the immediate Job permits breakaway.
type LaunchJobObservation struct {
	BeforeStart  JobObservation `json:"before_start"`
	AfterFailure JobObservation `json:"after_failure"`
	OuterJobs    string         `json:"outer_jobs"`
}

func (o LaunchJobObservation) Valid() bool {
	return o.OuterJobs == "unknown" && o.BeforeStart.Valid() && o.AfterFailure.Valid()
}

// No flags is different from three observed false flags. An unavailable query
// is never evidence that the process is outside a Job.
type JobObservation struct {
	State         string         `json:"state"`
	Flags         *JobLimitFlags `json:"flags,omitempty"`
	UnavailableAt string         `json:"unavailable_at,omitempty"`
	NativeError   uint32         `json:"native_error,omitempty"`
}

type JobLimitFlags struct {
	BreakawayOK       *bool `json:"breakaway_ok"`
	SilentBreakawayOK *bool `json:"silent_breakaway_ok"`
	KillOnJobClose    *bool `json:"kill_on_job_close"`
}

func (o JobObservation) Valid() bool {
	switch o.State {
	case "observed":
		return o.Flags != nil && o.Flags.BreakawayOK != nil && o.Flags.SilentBreakawayOK != nil && o.Flags.KillOnJobClose != nil && o.UnavailableAt == "" && o.NativeError == 0
	case "not_in_job":
		return o.Flags == nil && o.UnavailableAt == "" && o.NativeError == 0
	case "unavailable":
		return o.Flags == nil && (o.UnavailableAt == "membership" || o.UnavailableAt == "limits")
	default:
		return false
	}
}
