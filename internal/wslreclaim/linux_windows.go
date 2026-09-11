//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os/exec"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/reclamation"
)

// Both IDs come from enrollment checked under the continuation guard. No path,
// pool, executable, environment variable or command comes from Linux output.
func (r registration) linuxReclaimArguments(identity installationIdentity) ([]string, error) {
	if _, err := r.diskPath(); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return nil, err
	}
	if _, err := decodeInstallation(append(encoded, '\n'), r.ID); err != nil {
		return nil, err
	}
	return []string{"--distribution-id", r.ID.String(), "--user", "root", "--cd", "/", "--exec", "/usr/bin/env", "-i", "PATH=/usr/sbin:/usr/bin:/sbin:/bin", "/usr/local/bin/haco", "_reclaim-linux", identity.RegistrationID, identity.InstallationID}, nil
}

func (r registration) reclaimLinux(ctx context.Context, identity installationIdentity) (*reclamation.LinuxReport, error) {
	args, err := r.linuxReclaimArguments(identity)
	if err != nil {
		return nil, err
	}
	return receiveLinuxReport(ctx, func(ctx context.Context, output io.Writer) error {
		return r.runWSLArguments(ctx, args, output)
	})
}

// registrationOutput provides the same bounded 4 KiB pipe capture as the fixed
// identity reader. Child stderr and malformed stdout never enter saved evidence.
func receiveLinuxReport(ctx context.Context, run func(context.Context, io.Writer) error) (*reclamation.LinuxReport, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var output registrationOutput
	runErr := run(ctx, &output)
	report, err := decodeLinuxReport(output.data)
	if output.overflow || err != nil {
		return nil, errors.New("Linux reclamation result unavailable")
	}
	// Even complete-looking output cannot override a timeout, transport failure or
	// nonzero exit. A genuine exit 1 may carry a valid failed operation report.
	if err := ctx.Err(); err != nil {
		return report, err
	}
	if runErr != nil {
		var exit *exec.ExitError
		if errors.As(runErr, &exit) && exit.ExitCode() == 1 && !report.Complete() {
			return report, errors.New("Linux reclamation failed")
		}
		return report, errors.New("Linux reclamation invocation failed")
	}
	if !report.Complete() {
		return report, errors.New("Linux reclamation failed")
	}
	return report, nil
}

func decodeLinuxReport(raw []byte) (*reclamation.LinuxReport, error) {
	if len(raw) == 0 || len(raw) > 4096 {
		return nil, errors.New("invalid Linux result size")
	}
	var wire struct {
		ProtocolVersion int `json:"protocol_version"`
		reclamation.LinuxReport
	}
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, errors.New("invalid Linux result encoding")
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) || wire.ProtocolVersion != control.ProtocolVersion || wire.Validate() != nil {
		return nil, errors.New("unknown or invalid Linux result")
	}
	return &wire.LinuxReport, nil
}

// Persist attempted/unknown before crossing WSL. A lost result never permits
// replay or Windows shutdown. Persist the bounded report before shutdown so it
// remains readable even when WSL or the Windows worker is interrupted later.
func runRecordedLinux(ctx context.Context, records *operationStore, intent *operationRecord, run func(context.Context) (*reclamation.LinuxReport, error)) error {
	if intent.Version != 2 || intent.LinuxStarted {
		return errors.New("Linux operation cannot be replayed")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	started := *intent
	started.LinuxStarted = true
	if err := records.replacePending(*intent, started); err != nil {
		return err
	}
	*intent = started
	report, runErr := run(ctx)
	if report != nil {
		if err := report.Validate(); err != nil {
			return errors.New("invalid Linux reclamation report")
		}
		observed := *intent
		observed.Linux = report
		if err := records.replacePending(*intent, observed); err != nil {
			return err
		}
		*intent = observed
	}
	if runErr != nil {
		return runErr
	}
	if report == nil || !report.Complete() {
		return errors.New("Linux reclamation did not complete")
	}
	return ctx.Err()
}

func executeRecordedStages(ctx context.Context, records *operationStore, intent operationRecord,
	linux func(context.Context) (*reclamation.LinuxReport, error),
	windows func(context.Context) (continuationObservation, error),
) (result continuationObservation, err error) {
	defer func() { err = errors.Join(err, records.finish(intent, result, err)) }()
	if intent.Version == 2 {
		if err := runRecordedLinux(ctx, records, &intent, linux); err != nil {
			return result, err
		}
	}
	return windows(ctx)
}
