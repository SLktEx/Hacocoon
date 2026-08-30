package seedstats

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type DeleteReport struct {
	Reference           string            `json:"reference"`
	Digest              string            `json:"digest"`
	HostCache           string            `json:"host_cache"`
	SeedRebuildRequired bool              `json:"seed_rebuild_required"`
	RemovedEnvironments []string          `json:"removed_environments,omitempty"`
	SkippedEnvironments []string          `json:"skipped_environments,omitempty"`
	Failures            map[string]string `json:"failures,omitempty"`
}

type deleteTarget struct {
	Reference string
	Digest    string
}

func (t deleteTarget) key() string { return t.Reference + "@" + t.Digest }

// DeleteImage removes one immutable image identity from the Host Seed cache and
// records a Seed-deletion tombstone. Published Seed revisions are immutable, so
// the tombstone means the next Seed build must omit the image and publish a new
// revision. With allEnvironments, Hacocoon also removes the exact image from all
// managed Environments after re-validating that each mutable tag still points to
// the resolved digest. nerdctl --force is deliberately not used.
func (s *Service) DeleteImage(ctx context.Context, raw string, allEnvironments bool) (DeleteReport, error) {
	if s == nil || s.store == nil || s.runtime == nil || s.hostRunner == nil {
		return DeleteReport{}, core.ErrRuntimeUnavailable
	}
	target, err := s.resolveDeleteTarget(ctx, raw)
	if err != nil {
		return DeleteReport{}, err
	}
	report := DeleteReport{
		Reference: target.Reference,
		Digest:    target.Digest,
		HostCache: "not-present",
	}

	var environments []core.Environment
	if allEnvironments {
		environments, err = readEnvironments(s.environmentStatePath)
		if err != nil {
			return report, err
		}
		// Preflight every Environment before the first destructive action. This
		// catches moved tags and unavailable runtimes without producing an
		// avoidable partial all-Environment deletion.
		for _, environment := range environments {
			present, preflightErr := s.environmentHasTarget(ctx, environment, target)
			if preflightErr != nil {
				if report.Failures == nil {
					report.Failures = map[string]string{}
				}
				report.Failures[environment.Name] = preflightErr.Error()
				continue
			}
			if !present {
				report.SkippedEnvironments = append(report.SkippedEnvironments, environment.Name)
			}
		}
		if len(report.Failures) > 0 {
			sort.Strings(report.SkippedEnvironments)
			return report, fmt.Errorf("preflight OCI image deletion from all Environments: %w", core.ErrIncompatibleState)
		}
	}

	hostPresent, err := s.hostHasTarget(ctx, target)
	if err != nil {
		return report, err
	}

	if allEnvironments {
		for _, environment := range environments {
			if contains(report.SkippedEnvironments, environment.Name) {
				continue
			}
			removed, removeErr := s.removeEnvironmentTarget(ctx, environment, target)
			if removeErr != nil {
				if report.Failures == nil {
					report.Failures = map[string]string{}
				}
				report.Failures[environment.Name] = removeErr.Error()
				continue
			}
			if removed {
				report.RemovedEnvironments = append(report.RemovedEnvironments, environment.Name)
			} else {
				report.SkippedEnvironments = appendUnique(report.SkippedEnvironments, environment.Name)
			}
		}
		if len(report.Failures) > 0 {
			sort.Strings(report.RemovedEnvironments)
			sort.Strings(report.SkippedEnvironments)
			return report, fmt.Errorf("OCI image deletion completed only for some Environments: %w", core.ErrRecoveryRequired)
		}
	}

	if hostPresent {
		removed, removeErr := s.removeHostTarget(ctx, target)
		if removeErr != nil {
			if allEnvironments && len(report.RemovedEnvironments) > 0 {
				return report, errors.Join(removeErr, core.ErrRecoveryRequired)
			}
			return report, removeErr
		}
		if removed {
			report.HostCache = "removed"
		}
	}

	deletion := Deletion{Reference: target.Reference, Digest: target.Digest, DeletedAt: s.now().UTC()}
	if err := s.store.PutDeletion(ctx, deletion); err != nil {
		if report.HostCache == "removed" || len(report.RemovedEnvironments) > 0 {
			return report, errors.Join(fmt.Errorf("persist Seed deletion tombstone: %w", err), core.ErrRecoveryRequired)
		}
		return report, err
	}
	report.SeedRebuildRequired = true
	sort.Strings(report.RemovedEnvironments)
	sort.Strings(report.SkippedEnvironments)
	return report, nil
}

func (s *Service) resolveDeleteTarget(ctx context.Context, raw string) (deleteTarget, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return deleteTarget{}, core.ErrInvalidArgument
	}
	if cut := strings.LastIndexByte(raw, '@'); cut >= 0 {
		reference, digest := raw[:cut], strings.ToLower(raw[cut+1:])
		if err := validateReference(reference); err != nil {
			return deleteTarget{}, err
		}
		if !digestPattern.MatchString(digest) {
			return deleteTarget{}, fmt.Errorf("invalid OCI digest %q: %w", digest, core.ErrInvalidArgument)
		}
		return deleteTarget{Reference: reference, Digest: digest}, nil
	}
	if err := validateReference(raw); err != nil {
		return deleteTarget{}, err
	}
	snapshots, err := s.store.List(ctx)
	if err != nil {
		return deleteTarget{}, err
	}
	digests := map[string]struct{}{}
	for _, snapshot := range snapshots {
		for _, image := range snapshot.Images {
			if image.Reference() == raw && image.Digest != "" {
				digests[image.Digest] = struct{}{}
			}
		}
	}
	if len(digests) == 0 {
		return deleteTarget{}, fmt.Errorf("OCI image %q has no observed immutable digest: %w", raw, core.ErrNotFound)
	}
	if len(digests) > 1 {
		return deleteTarget{}, fmt.Errorf("OCI image %q has multiple observed digests; specify reference@sha256:... explicitly: %w", raw, core.ErrIncompatibleState)
	}
	for digest := range digests {
		return deleteTarget{Reference: raw, Digest: digest}, nil
	}
	panic("unreachable")
}

func (s *Service) environmentHasTarget(ctx context.Context, environment core.Environment, target deleteTarget) (bool, error) {
	result, err := s.runtime.ExecEnvironment(ctx, environment.RuntimeRef, core.ExecutionRequest{Argv: imageListArgv()})
	if err != nil || result.ExitCode != 0 {
		return false, commandFailure("list Environment OCI images", result.Stderr, err, result.ExitCode)
	}
	images, err := parseNerdctlImages(result.Stdout)
	if err != nil {
		return false, err
	}
	return targetPresent(images, target)
}

func (s *Service) removeEnvironmentTarget(ctx context.Context, environment core.Environment, target deleteTarget) (bool, error) {
	present, err := s.environmentHasTarget(ctx, environment, target)
	if err != nil || !present {
		return false, err
	}
	result, execErr := s.runtime.ExecEnvironment(ctx, environment.RuntimeRef, core.ExecutionRequest{Argv: []string{"nerdctl", "rmi", target.Reference}})
	if execErr != nil || result.ExitCode != 0 {
		if looksNotFound(result.Stderr) {
			return false, nil
		}
		return false, commandFailure("remove Environment OCI image", result.Stderr, execErr, result.ExitCode)
	}
	return true, nil
}

func (s *Service) hostHasTarget(ctx context.Context, target deleteTarget) (bool, error) {
	result, err := s.hostRunner.Run(ctx, "nerdctl", append([]string{"--namespace", s.seedNamespace}, imageListArgv()[1:]...)...)
	if err != nil || result.ExitCode != 0 {
		if looksNamespaceNotFound(result.Stderr) {
			return false, nil
		}
		return false, commandFailure("list Host Seed-cache OCI images", result.Stderr, err, result.ExitCode)
	}
	images, err := parseNerdctlImages(result.Stdout)
	if err != nil {
		return false, err
	}
	return targetPresent(images, target)
}

func (s *Service) removeHostTarget(ctx context.Context, target deleteTarget) (bool, error) {
	present, err := s.hostHasTarget(ctx, target)
	if err != nil || !present {
		return false, err
	}
	result, runErr := s.hostRunner.Run(ctx, "nerdctl", "--namespace", s.seedNamespace, "rmi", target.Reference)
	if runErr != nil || result.ExitCode != 0 {
		if looksNotFound(result.Stderr) {
			return false, nil
		}
		return false, commandFailure("remove Host Seed-cache OCI image", result.Stderr, runErr, result.ExitCode)
	}
	return true, nil
}

func imageListArgv() []string {
	return []string{"nerdctl", "images", "--format", "{{.Repository}}\t{{.Tag}}\t{{.Digest}}"}
}

func targetPresent(images []Image, target deleteTarget) (bool, error) {
	for _, image := range images {
		if image.Reference() != target.Reference {
			continue
		}
		if image.Digest == target.Digest {
			return true, nil
		}
		if image.Digest != "" {
			return false, fmt.Errorf("OCI reference %q now points to %s instead of deletion target %s: %w", target.Reference, image.Digest, target.Digest, core.ErrIncompatibleState)
		}
	}
	return false, nil
}

func commandFailure(action, stderr string, err error, exitCode int) error {
	reason := strings.TrimSpace(stderr)
	if reason == "" && err != nil {
		reason = err.Error()
	}
	if reason == "" {
		reason = fmt.Sprintf("exit code %d", exitCode)
	}
	return fmt.Errorf("%s: %s: %w", action, reason, core.ErrRuntimeUnavailable)
}

func looksNotFound(stderr string) bool {
	value := strings.ToLower(stderr)
	return strings.Contains(value, "not found") || strings.Contains(value, "no such image") || strings.Contains(value, "does not exist")
}

func looksNamespaceNotFound(stderr string) bool {
	value := strings.ToLower(stderr)
	return strings.Contains(value, "namespace") && (strings.Contains(value, "not found") || strings.Contains(value, "does not exist"))
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func appendUnique(values []string, value string) []string {
	if contains(values, value) {
		return values
	}
	return append(values, value)
}

var _ host.Runner = host.ExecRunner{}
