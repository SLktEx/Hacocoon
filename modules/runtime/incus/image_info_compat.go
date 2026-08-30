package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	guestSystemctlSettleTimeout  = 30 * time.Second
	guestSystemctlRetryInterval = 100 * time.Millisecond
)

// imageInfoCompatRunner centralizes the narrow Incus CLI compatibility and
// guest-readiness paths needed by Hacocoon. The historical name is retained
// because image-info compatibility was the first responsibility of this
// wrapper.
//
// Two exact machine-readable surfaces are handled here:
//   - `incus image info ... --format json`: prefer the modern CLI and fall back
//     to strict legacy Fingerprint parsing only when the modern command fails.
//   - `incus config show <instance> --project <project> --expanded --format json`:
//     prefer the Incus instance API and synthesize the legacy `devices` JSON
//     shape from `expanded_devices`; fall back to the CLI only when `query` is
//     unsupported (exit 2) or returns an empty successful response.
//
// Exact guest `systemctl show -p ActiveState` inspections are also allowed to
// cross the short Incus boot boundary. They retry only while the system bus is
// not created yet or while systemd reports a transitional unit state. Unknown
// errors and settled states are returned immediately, so readiness polling
// cannot hide a failed/missing unit.
//
// Malformed, ambiguous, or truncated machine-readable state always fails
// closed; it never causes a compatibility fallback.
type imageInfoCompatRunner struct {
	next host.Runner
}

func wrapImageInfoCompatRunner(next host.Runner) host.Runner {
	if next == nil {
		return next
	}
	if _, ok := next.(*imageInfoCompatRunner); ok {
		return next
	}
	return &imageInfoCompatRunner{next: next}
}

func (r *imageInfoCompatRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	if r == nil || r.next == nil {
		return host.Result{ExitCode: -1}, core.ErrRuntimeUnavailable
	}
	if apiPath, ok := expandedConfigShowAPIPath(name, args); ok {
		return r.runExpandedConfigShow(ctx, name, apiPath, args)
	}
	if guestSystemctlActiveStateCandidate(name, args) {
		return r.runGuestSystemctlActiveState(ctx, name, args)
	}
	if !legacyImageInfoCandidate(name, args) {
		return r.next.Run(ctx, name, args...)
	}

	result, err := r.next.Run(ctx, name, args...)
	if err == nil && result.ExitCode == 0 {
		return result, nil
	}
	jsonErr := commandResultError(result, err)

	legacyArgs := append([]string(nil), args[:len(args)-2]...)
	legacyResult, legacyErr := r.next.Run(ctx, name, legacyArgs...)
	if legacyErr != nil || legacyResult.ExitCode != 0 {
		return legacyResult, errors.Join(
			fmt.Errorf("machine-readable Incus image info unavailable: %w", jsonErr),
			fmt.Errorf("legacy Incus image info failed: %w", commandResultError(legacyResult, legacyErr)),
		)
	}
	if legacyResult.StdoutTruncated {
		return host.Result{ExitCode: -1}, fmt.Errorf("legacy Incus image info output was truncated: %w", core.ErrIncompatibleState)
	}
	fingerprint, parseErr := parseLegacyImageFingerprint(legacyResult.Stdout)
	if parseErr != nil {
		return host.Result{ExitCode: -1}, parseErr
	}
	encoded, marshalErr := json.Marshal(struct {
		Fingerprint string `json:"fingerprint"`
	}{Fingerprint: fingerprint})
	if marshalErr != nil {
		return host.Result{ExitCode: -1}, fmt.Errorf("encode compatible Incus image info: %w", marshalErr)
	}
	legacyResult.Stdout = string(encoded)
	legacyResult.ExitCode = 0
	return legacyResult, nil
}

func (r *imageInfoCompatRunner) runExpandedConfigShow(ctx context.Context, name, apiPath string, legacyArgs []string) (host.Result, error) {
	queryResult, queryErr := r.next.Run(ctx, name, "query", apiPath)
	if queryErr == nil && queryResult.ExitCode == 0 && strings.TrimSpace(queryResult.Stdout) != "" {
		if queryResult.StdoutTruncated {
			return host.Result{ExitCode: -1}, fmt.Errorf("Incus instance API output was truncated: %w", core.ErrIncompatibleState)
		}
		var state struct {
			ExpandedDevices map[string]map[string]string `json:"expanded_devices"`
		}
		if err := json.Unmarshal([]byte(queryResult.Stdout), &state); err != nil {
			return host.Result{ExitCode: -1}, fmt.Errorf("decode Incus instance API state: %w", core.ErrIncompatibleState)
		}
		if state.ExpandedDevices == nil {
			return host.Result{ExitCode: -1}, fmt.Errorf("Incus instance API omitted expanded_devices: %w", core.ErrIncompatibleState)
		}
		encoded, err := json.Marshal(struct {
			Devices map[string]map[string]string `json:"devices"`
		}{Devices: state.ExpandedDevices})
		if err != nil {
			return host.Result{ExitCode: -1}, fmt.Errorf("encode compatible expanded Incus config: %w", err)
		}
		queryResult.Stdout = string(encoded)
		queryResult.ExitCode = 0
		return queryResult, nil
	}

	queryUnsupported := queryResult.ExitCode == 2 || (queryErr == nil && queryResult.ExitCode == 0 && strings.TrimSpace(queryResult.Stdout) == "")
	if !queryUnsupported {
		return queryResult, fmt.Errorf("inspect expanded Incus instance config via API: %w", commandResultError(queryResult, queryErr))
	}

	legacyResult, legacyErr := r.next.Run(ctx, name, legacyArgs...)
	if legacyErr != nil || legacyResult.ExitCode != 0 {
		return legacyResult, errors.Join(
			fmt.Errorf("Incus instance API query unavailable: %w", commandResultError(queryResult, queryErr)),
			fmt.Errorf("legacy expanded Incus config failed: %w", commandResultError(legacyResult, legacyErr)),
		)
	}
	return legacyResult, nil
}

func (r *imageInfoCompatRunner) runGuestSystemctlActiveState(ctx context.Context, name string, args []string) (host.Result, error) {
	waitCtx, cancel := context.WithTimeout(ctx, guestSystemctlSettleTimeout)
	defer cancel()

	var lastResult host.Result
	var lastErr error
	for {
		result, err := r.next.Run(waitCtx, name, args...)
		lastResult, lastErr = result, err
		state := strings.TrimSpace(result.Stdout)

		if err == nil {
			if !transitionalSystemdUnitState(state) {
				return result, nil
			}
		} else if !guestSystemdBusNotReady(result.Stderr) {
			return result, enrichGuestSystemctlError(result, err)
		}

		timer := time.NewTimer(guestSystemctlRetryInterval)
		select {
		case <-waitCtx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			if lastErr != nil {
				return lastResult, errors.Join(enrichGuestSystemctlError(lastResult, lastErr), waitCtx.Err(), core.ErrRuntimeUnavailable)
			}
			return lastResult, errors.Join(
				fmt.Errorf("guest systemd unit remained in transitional state %q", strings.TrimSpace(lastResult.Stdout)),
				waitCtx.Err(),
				core.ErrRuntimeUnavailable,
			)
		case <-timer.C:
		}
	}
}

func enrichGuestSystemctlError(result host.Result, err error) error {
	reason := strings.TrimSpace(result.Stderr)
	if reason != "" {
		return fmt.Errorf("guest systemctl show failed: %s: %w", reason, err)
	}
	return err
}

func guestSystemdBusNotReady(stderr string) bool {
	reason := strings.ToLower(strings.TrimSpace(stderr))
	return strings.Contains(reason, "failed to connect to system scope bus") && strings.Contains(reason, "no such file or directory") ||
		strings.Contains(reason, "failed to connect to bus") && strings.Contains(reason, "no such file or directory")
}

func transitionalSystemdUnitState(state string) bool {
	switch state {
	case "activating", "deactivating", "reloading":
		return true
	default:
		return false
	}
}

func expandedConfigShowAPIPath(name string, args []string) (string, bool) {
	if name != "incus" || len(args) != 8 {
		return "", false
	}
	if args[0] != "config" || args[1] != "show" || strings.TrimSpace(args[2]) == "" {
		return "", false
	}
	if args[3] != "--project" || strings.TrimSpace(args[4]) == "" || args[5] != "--expanded" || args[6] != "--format" || args[7] != "json" {
		return "", false
	}
	return "/1.0/instances/" + url.PathEscape(args[2]) + "?project=" + url.QueryEscape(args[4]), true
}

func guestSystemctlActiveStateCandidate(name string, args []string) bool {
	if name != "incus" || len(args) < 11 || args[0] != "exec" || strings.TrimSpace(args[1]) == "" {
		return false
	}
	separator := -1
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || len(args) != separator+7 {
		return false
	}
	return args[separator+1] == "systemctl" &&
		args[separator+2] == "show" &&
		args[separator+3] == "-p" &&
		args[separator+4] == "ActiveState" &&
		args[separator+5] == "--value" &&
		strings.TrimSpace(args[separator+6]) != ""
}

func legacyImageInfoCandidate(name string, args []string) bool {
	if name != "incus" || len(args) < 5 {
		return false
	}
	if args[0] != "image" || args[1] != "info" {
		return false
	}
	return args[len(args)-2] == "--format" && args[len(args)-1] == "json"
}

func parseLegacyImageFingerprint(output string) (string, error) {
	fingerprint := ""
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "Fingerprint:") {
			continue
		}
		value := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "Fingerprint:")))
		if fingerprint != "" {
			return "", fmt.Errorf("legacy Incus image info contains multiple fingerprints: %w", core.ErrIncompatibleState)
		}
		fingerprint = value
	}
	if !baseFingerprintPattern.MatchString(fingerprint) {
		return "", fmt.Errorf("legacy Incus image info returned invalid fingerprint: %w", core.ErrIncompatibleState)
	}
	return fingerprint, nil
}

var _ host.Runner = (*imageInfoCompatRunner)(nil)
