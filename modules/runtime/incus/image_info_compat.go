package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// imageInfoCompatRunner centralizes the narrow Incus CLI compatibility paths
// needed by Hacocoon. The historical name is retained because image-info
// compatibility was the first responsibility of this wrapper.
//
// Two exact machine-readable surfaces are handled here:
//   - `incus image info ... --format json`: prefer the modern CLI and fall back
//     to strict legacy Fingerprint parsing only when the modern command fails.
//   - `incus config show <instance> --project <project> --expanded --format json`:
//     prefer the Incus instance API and synthesize the legacy `devices` JSON
//     shape from `expanded_devices`; fall back to the CLI only when `query` is
//     unsupported (exit 2) or returns an empty successful response.
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
