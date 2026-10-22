package wslreclaim

import (
	"context"
	"errors"
	"syscall"

	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

// Observe only around the existing single Start call. No query result gates
// launch, changes its flags/context, retries it, or establishes worker absence.
func startObservedWorker(ctx context.Context, start func() error, observe func() reclamation.JobObservation) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	before := observe()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := start(); err != nil {
		return &workerLaunchError{stage: "process_start", err: err, job: &reclamation.LaunchJobObservation{
			BeforeStart: before, AfterFailure: observe(), OuterJobs: "unknown",
		}}
	}
	return nil
}

// Query results contain only fixed states, three flags and a DWORD error code.
func observeJob(membership func() (bool, error), limits func() (uint32, error)) reclamation.JobObservation {
	inJob, err := membership()
	if err != nil {
		return unavailableJob("membership", err)
	}
	if !inJob {
		return reclamation.JobObservation{State: "not_in_job"}
	}
	flags, err := limits()
	if err != nil {
		return unavailableJob("limits", err)
	}
	breakaway, silent, kill := flags&0x800 != 0, flags&0x1000 != 0, flags&0x2000 != 0
	return reclamation.JobObservation{State: "observed", Flags: &reclamation.JobLimitFlags{
		BreakawayOK: &breakaway, SilentBreakawayOK: &silent, KillOnJobClose: &kill,
	}}
}

func unavailableJob(query string, err error) reclamation.JobObservation {
	o := reclamation.JobObservation{State: "unavailable", UnavailableAt: query}
	var code syscall.Errno
	if errors.As(err, &code) && uint64(code) <= 0xffffffff {
		o.NativeError = uint32(code)
	}
	return o
}

// Fixed diagnostics for the owning CLI boundary; never expose child output.
type workerLaunchError struct {
	stage string
	err   error
	job   *reclamation.LaunchJobObservation
}

func (e *workerLaunchError) Error() string {
	return "Windows worker " + e.stage + " failed: " + e.err.Error()
}
func (e *workerLaunchError) Unwrap() error { return e.err }
func WorkerLaunchFailure(err error) reclamation.InvocationFailure {
	result := reclamation.InvocationFailure{Phase: "launch", Stage: "other"}
	var failure *workerLaunchError
	if errors.As(err, &failure) {
		switch failure.stage {
		case "pin_executable", "process_start", "readiness":
			result.Stage = failure.stage
		}
		if result.Stage == "process_start" && failure.job != nil && failure.job.Valid() {
			result.Job = failure.job
		}
	}
	var code syscall.Errno
	if errors.As(err, &code) {
		result.NativeError = uint32(code)
	}
	return result
}
