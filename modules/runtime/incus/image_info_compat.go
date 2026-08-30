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
		return "", fmt.Errorf("Incus image lookup requires a remote-qualified source %q: %w", source, core.ErrUnsupported)
	}

	args := []string{"image", "list", remote + ":", identifier, "--format", "csv", "-c", "L,F"}
	if project != "" {
		args = append(args, "--project", project)
	}
	result, err := p.runner.Run(ctx, "incus", args...)
	if err != nil {
		return "", err
	}

	matches, err := matchingImageFingerprints(result.Stdout, source, identifier, 2)
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("Incus image %q: %w", source, core.ErrNotFound)
	}
	if len(matches) == 1 {
		for fingerprint := range matches {
			return fingerprint, nil
		}
	}

	// The public images remote can publish the same alias for a system
	// container and a VM. Incus 7 doesn't expose a machine-readable
	// `image info --format=json`, so only perform the extra type lookup when the
	// compatibility list path is actually ambiguous. This preserves the older
	// Incus CLI contract for the common single-match case while ensuring the
	// Hacocoon runtime pins the system-container image.
	return p.containerFingerprintFromList(ctx, remote, identifier, project, source, matches)
}

func (p *BaseProvider) containerFingerprintFromList(ctx context.Context, remote, identifier, project, source string, candidates map[string]struct{}) (string, error) {
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
		if strings.ToLower(strings.TrimSpace(record[2])) != "container" {
			continue
		}
		fingerprint := strings.ToLower(strings.TrimSpace(record[1]))
		if _, ok := candidates[fingerprint]; !ok {
			continue
		}
		if imageListRecordMatches(record[0], fingerprint, identifier, identifierLower, identifierIsFingerprint) {
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

func matchingImageFingerprints(raw, source, identifier string, columns int) (map[string]struct{}, error) {
	reader := csv.NewReader(strings.NewReader(raw))
	matches := map[string]struct{}{}
	identifierLower := strings.ToLower(identifier)
	identifierIsFingerprint := isHexFingerprintPrefix(identifierLower)

	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("decode Incus image list for %q: %w", source, core.ErrIncompatibleState)
		}
		if len(record) != columns {
			return nil, fmt.Errorf("unexpected Incus image list columns for %q: %w", source, core.ErrIncompatibleState)
		}
		fingerprint := strings.ToLower(strings.TrimSpace(record[1]))
		if !baseFingerprintPattern.MatchString(fingerprint) {
			continue
		}
		if imageListRecordMatches(record[0], fingerprint, identifier, identifierLower, identifierIsFingerprint) {
			matches[fingerprint] = struct{}{}
		}
	}
	return matches, nil
}

func imageListRecordMatches(aliases, fingerprint, identifier, identifierLower string, identifierIsFingerprint bool) bool {
	if identifierIsFingerprint {
		return strings.HasPrefix(fingerprint, identifierLower)
	}
	for _, alias := range strings.Split(aliases, "\n") {
		if strings.TrimSpace(alias) == identifier {
			return true
		}
	}
	return false
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
