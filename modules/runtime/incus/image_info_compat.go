package incus

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// imageFingerprint resolves the immutable container image fingerprint behind
// an Incus source. `incus image info` has no stable machine-readable --format
// flag across the supported Incus releases, while `image list` exposes stable
// CSV columns. Resolving through the list also lets us explicitly select the
// system-container image when a remote publishes the same alias for both a
// container and a virtual-machine image.
func (p *BaseProvider) imageFingerprint(ctx context.Context, source, project string) (string, error) {
	return p.imageFingerprintFromList(ctx, source, project)
}

func (p *BaseProvider) imageFingerprintFromList(ctx context.Context, source, project string) (string, error) {
	remote, identifier, ok := strings.Cut(source, ":")
	if !ok || remote == "" || identifier == "" {
		return "", fmt.Errorf("Incus image lookup requires a remote-qualified source %q: %w", source, core.ErrUnsupported)
	}

	args := []string{"image", "list", remote + ":", identifier, "--format", "csv", "-c", "L,F,T"}
	if project != "" {
		args = append(args, "--project", project)
	}
	result, err := p.runner.Run(ctx, "incus", args...)
	if err != nil {
		return "", err
	}

	reader := csv.NewReader(strings.NewReader(result.Stdout))
	matches := map[string]struct{}{}
	identifierLower := strings.ToLower(identifier)
	identifierIsFingerprint := isHexFingerprintPrefix(identifierLower)

	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", fmt.Errorf("decode Incus image list for %q: %w", source, core.ErrIncompatibleState)
		}
		if len(record) != 3 {
			return "", fmt.Errorf("unexpected Incus image list columns for %q: %w", source, core.ErrIncompatibleState)
		}

		// The public images remote commonly publishes the same alias for a
		// container and a VM. Hacocoon's Incus runtime is system-container-only,
		// so a VM fingerprint must never make the Base ambiguous or be selected.
		if strings.ToLower(strings.TrimSpace(record[2])) != "container" {
			continue
		}

		fingerprint := strings.ToLower(strings.TrimSpace(record[1]))
		if !baseFingerprintPattern.MatchString(fingerprint) {
			continue
		}

		matched := false
		if identifierIsFingerprint {
			matched = strings.HasPrefix(fingerprint, identifierLower)
		} else {
			for _, alias := range strings.Split(record[0], "\n") {
				if strings.TrimSpace(alias) == identifier {
					matched = true
					break
				}
		}
		if matched {
			matches[fingerprint] = struct{}{}
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("Incus container image %q: %w", source, core.ErrNotFound)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("Incus container image %q resolved to multiple fingerprints: %w", source, core.ErrIncompatibleState)
	}
	for fingerprint := range matches {
		return fingerprint, nil
	}
	panic("unreachable")
}

func isHexFingerprintPrefix(value string) bool {
	if len(value) < 12 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return true
}
