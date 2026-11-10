package cli

import (
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/SLktEx/Hacocoon/internal/client/reclaim"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/storage/reclamation"
)

func writeReclaimInvocationFailure(out io.Writer, err error) {
	var failure *reclaimclient.InvocationError
	if !errors.As(err, &failure) || !failure.Failure.Valid() {
		return
	}
	f := failure.Failure
	_, _ = fmt.Fprintln(out, cliMessage("reclaim.failure", cliMessage("reclaim.phase."+f.Phase), cliMessage("reclaim.stage."+f.Stage)))
	if f.NativeError != 0 {
		_, _ = fmt.Fprintln(out, cliMessage("reclaim.native_error", f.NativeError))
	}
	if f.Job != nil {
		_, _ = fmt.Fprintln(out, cliMessage("reclaim.job_scope"))
		// Helper stderr is deliberately discarded by the bridge. Only validated,
		// fixed observations reach the owning CLI logger, never the original error.
		if logger, err := logging.NewFromEnv(out); err == nil {
			logger.Info("Windows launcher Job observation", "component", "cli", "operation", "reclaim_storage",
				"scope", "launcher_immediate_job", "outer_jobs", f.Job.OuterJobs,
				"before_start", reclaimJobLogValue(f.Job.BeforeStart), "after_failure", reclaimJobLogValue(f.Job.AfterFailure))
		}
	}
	key := "reclaim.inspect"
	switch f.Stage {
	case "registration", "enrollment", "installation", "binding":
		key = "reclaim.installation"
	case "disk_access", "record_access", "windows_owner":
		key = "reclaim.access"
	case "exclusion", "intent":
		key = "reclaim.existing"
	}
	_, _ = fmt.Fprintln(out, cliMessage(key))
}

// Only validated protocol enums map to trusted display keys; wire values stay unchanged.
func reclamationValue(value string) string {
	switch value {
	case "complete":
		return cliMessage("reclaim.value.complete")
	case "failed":
		return cliMessage("reclaim.value.failed")
	case "pending":
		return cliMessage("reclaim.value.pending")
	case "interrupted":
		return cliMessage("reclaim.value.interrupted")
	case "skipped":
		return cliMessage("reclaim.value.skipped")
	case "stop":
		return cliMessage("reclaim.value.stop")
	case "compact":
		return cliMessage("reclaim.value.compact")
	case "compact_attached":
		return cliMessage("reclaim.value.compact_attached")
	case "resume":
		return cliMessage("reclaim.value.resume")
	case "identity_unavailable":
		return cliMessage("reclaim.value.identity_unavailable")
	case "identity_changed":
		return cliMessage("reclaim.value.identity_changed")
	case "pool_unavailable":
		return cliMessage("reclaim.value.pool_unavailable")
	case "pool_trim_failed":
		return cliMessage("reclaim.value.pool_trim_failed")
	case "outer_trim_failed":
		return cliMessage("reclaim.value.outer_trim_failed")
	case "cleanup_failed":
		return cliMessage("reclaim.value.cleanup_failed")
	case "canceled":
		return cliMessage("reclaim.value.canceled")
	default:
		return value
	}
}

func reclamationFlag(value bool) string {
	if value {
		return cliMessage("reclaim.value.true")
	}
	return cliMessage("reclaim.value.false")
}

func reclaimJobLogValue(o reclamation.JobObservation) slog.Value {
	fields := []slog.Attr{slog.String("state", o.State)}
	if o.Flags != nil {
		fields = append(fields, slog.Bool("breakaway_ok", *o.Flags.BreakawayOK), slog.Bool("silent_breakaway_ok", *o.Flags.SilentBreakawayOK), slog.Bool("kill_on_job_close", *o.Flags.KillOnJobClose))
	}
	if o.UnavailableAt != "" {
		fields = append(fields, slog.String("unavailable_at", o.UnavailableAt))
	}
	if o.NativeError != 0 {
		fields = append(fields, slog.Uint64("native_error", uint64(o.NativeError)))
	}
	return slog.GroupValue(fields...)
}
