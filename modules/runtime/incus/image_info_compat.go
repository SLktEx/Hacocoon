package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// imageInfoCompatRunner preserves the machine-readable image-info path used by
// newer Incus releases while supporting the Incus 6.0 CLI shipped by Ubuntu
// 24.04, whose `incus image info` command does not support `--format json`.
//
// The fallback is deliberately narrow: it is used only for an exact
// `incus image info ... --format json` command after that command fails. The
// legacy human-readable output must contain exactly one valid SHA-256-sized
// Fingerprint field before we synthesize the minimal JSON object expected by
// Hacocoon. Ambiguous, malformed, or truncated output fails closed.
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
