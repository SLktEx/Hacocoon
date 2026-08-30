package incus

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (p *BaseProvider) imageFingerprint(ctx context.Context, source, project string) (string, error) {
	args := []string{"image", "info", source}
	if project != "" {
		args = append(args, "--project", project)
	}
	args = append(args, "--format", "json")

	result, err := p.runner.Run(ctx, "incus", args...)
	if err == nil {
		var info struct {
			Fingerprint string `json:"fingerprint"`
		}
		if decodeErr := json.Unmarshal([]byte(result.Stdout), &info); decodeErr != nil {
			return "", fmt.Errorf("decode Incus image info for %q: %w", source, core.ErrIncompatibleState)
		}
		fingerprint := strings.ToLower(strings.TrimSpace(info.Fingerprint))
		if !baseFingerprintPattern.MatchString(fingerprint) {
			return "", fmt.Errorf("invalid Incus image fingerprint for %q: %w", source, core.ErrIncompatibleState)
		}
		return fingerprint, nil
	}

	if !strings.Contains(strings.ToLower(result.Stderr), "unknown flag: --format") {
		return "", err
	}

	return p.imageFingerprintFromList(ctx, source, project)
}

func (p *BaseProvider) imageFingerprintFromList(ctx context.Context, source, project string) (string, error) {
	remote, identifier, ok := strings.Cut(source, ":")
	if !ok || remote == "" || identifier == "" {
		return "", fmt.Errorf("Incus 6.0 image lookup requires a remote-qualified source %q: %w", source, core.ErrUnsupported)
	}

	args := []string{"image", "list", remote + ":", identifier, "--format", "csv", "-c", "L,F"}
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
		if len(record) != 2 {
			return "", fmt.Errorf("unexpected Incus image list columns for %q: %w", source, core.ErrIncompatibleState)
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
		}
		if matched {
			matches[fingerprint] = struct{}{}
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("Incus image %q: %w", source, core.ErrNotFound)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("Incus image %q resolved to multiple fingerprints: %w", source, core.ErrIncompatibleState)
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
