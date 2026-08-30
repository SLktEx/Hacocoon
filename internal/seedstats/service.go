package seedstats

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	DefaultSampleMaxAge          = 6 * time.Hour
	DefaultRecommendationWindow = 30 * 24 * time.Hour
	DefaultAutoPromotionPercent = 10
)

type environmentExecutor interface {
	ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
}

type Service struct {
	runtime              environmentExecutor
	environmentStatePath string
	store                *Store
	now                  func() time.Time
}

type SampleReport struct {
	Sampled  int               `json:"sampled"`
	Fresh    int               `json:"fresh"`
	Failed   int               `json:"failed"`
	Failures map[string]string `json:"failures,omitempty"`
}

type Recommendation struct {
	Reference    string    `json:"reference"`
	Digest       string    `json:"digest"`
	Environments int       `json:"environments"`
	Percent      float64   `json:"percent"`
	LastSeen     time.Time `json:"last_seen"`
	AutoPromote  bool      `json:"auto_promote"`
}

func New(runtime environmentExecutor, environmentStatePath string, store *Store) (*Service, error) {
	if runtime == nil || strings.TrimSpace(environmentStatePath) == "" || store == nil {
		return nil, core.ErrInvalidArgument
	}
	return &Service{
		runtime:              runtime,
		environmentStatePath: environmentStatePath,
		store:                store,
		now:                  time.Now,
	}, nil
}

// SampleAll opportunistically records OCI image usage from Hacocoon-managed
// Environments. A positive maxAge skips snapshots that are still fresh. A zero
// maxAge forces a new sample attempt for every known Environment.
func (s *Service) SampleAll(ctx context.Context, maxAge time.Duration) (SampleReport, error) {
	if s == nil || s.runtime == nil || s.store == nil {
		return SampleReport{}, core.ErrInvalidArgument
	}
	if maxAge < 0 {
		return SampleReport{}, core.ErrInvalidArgument
	}
	environments, err := readEnvironments(s.environmentStatePath)
	if err != nil {
		return SampleReport{}, err
	}
	report := SampleReport{Failures: map[string]string{}}
	now := s.now().UTC()
	for _, environment := range environments {
		if maxAge > 0 {
			if snapshot, err := s.store.Get(ctx, environment.Name); err == nil && now.Sub(snapshot.SampledAt) < maxAge {
				report.Fresh++
				continue
			} else if err != nil && !errors.Is(err, core.ErrNotFound) {
				return report, err
			}
		}

		result, execErr := s.runtime.ExecEnvironment(ctx, environment.RuntimeRef, core.ExecutionRequest{Argv: []string{
			"nerdctl", "images", "--format", "{{.Repository}}\t{{.Tag}}\t{{.Digest}}",
		}})
		if execErr != nil || result.ExitCode != 0 {
			report.Failed++
			reason := strings.TrimSpace(result.Stderr)
			if reason == "" && execErr != nil {
				reason = execErr.Error()
			}
			if reason == "" {
				reason = fmt.Sprintf("nerdctl images exited %d", result.ExitCode)
			}
			report.Failures[environment.Name] = reason
			continue
		}
		images, parseErr := parseNerdctlImages(result.Stdout)
		if parseErr != nil {
			report.Failed++
			report.Failures[environment.Name] = parseErr.Error()
			continue
		}
		if err := s.store.Put(ctx, Snapshot{Environment: environment.Name, SampledAt: now, Images: images}); err != nil {
			return report, err
		}
		report.Sampled++
	}
	if len(report.Failures) == 0 {
		report.Failures = nil
	}
	return report, nil
}

func (s *Service) Recommend(ctx context.Context, window time.Duration) ([]Recommendation, error) {
	if s == nil || s.store == nil || window <= 0 {
		return nil, core.ErrInvalidArgument
	}
	snapshots, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	cutoff := s.now().UTC().Add(-window)
	type aggregate struct {
		reference string
		digest    string
		count     int
		lastSeen  time.Time
	}
	aggregates := map[string]*aggregate{}
	denominator := 0
	for _, snapshot := range snapshots {
		if snapshot.SampledAt.Before(cutoff) {
			continue
		}
		denominator++
		seen := map[string]struct{}{}
		for _, image := range snapshot.Images {
			// A Seed recommendation must have immutable identity. Images whose
			// digest could not be observed are kept in telemetry but are not
			// eligible for recommendation or automatic promotion.
			if image.Digest == "" {
				continue
			}
			key := image.Reference() + "@" + image.Digest
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			a := aggregates[key]
			if a == nil {
				a = &aggregate{reference: image.Reference(), digest: image.Digest}
				aggregates[key] = a
			}
			a.count++
			if snapshot.SampledAt.After(a.lastSeen) {
				a.lastSeen = snapshot.SampledAt
			}
		}
	}
	if denominator == 0 {
		return []Recommendation{}, nil
	}
	result := make([]Recommendation, 0, len(aggregates))
	for _, aggregate := range aggregates {
		result = append(result, Recommendation{
			Reference:    aggregate.reference,
			Digest:       aggregate.digest,
			Environments: aggregate.count,
			Percent:      float64(aggregate.count) * 100 / float64(denominator),
			LastSeen:     aggregate.lastSeen,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Environments != result[j].Environments {
			return result[i].Environments > result[j].Environments
		}
		if result[i].LastSeen != result[j].LastSeen {
			return result[i].LastSeen.After(result[j].LastSeen)
		}
		if result[i].Reference != result[j].Reference {
			return result[i].Reference < result[j].Reference
		}
		return result[i].Digest < result[j].Digest
	})
	markAutoPromotions(result, DefaultAutoPromotionPercent)
	return result, nil
}

// markAutoPromotions marks the top ceil(percent%) of ranked recommendations.
// If at least one eligible recommendation exists, at least one is promoted.
// This only selects content for the next Seed build; it does not mutate an
// existing Seed or Environment.
func markAutoPromotions(recommendations []Recommendation, percent int) {
	if len(recommendations) == 0 || percent <= 0 {
		return
	}
	if percent > 100 {
		percent = 100
	}
	count := (len(recommendations)*percent + 99) / 100
	if count < 1 {
		count = 1
	}
	if count > len(recommendations) {
		count = len(recommendations)
	}
	for i := range recommendations {
		recommendations[i].AutoPromote = i < count
	}
}

func readEnvironments(path string) ([]core.Environment, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []core.Environment{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read environment state for seed telemetry: %w", err)
	}
	var state struct {
		Environments map[string]core.Environment `json:"environments"`
	}
	if err := json.Unmarshal(contents, &state); err != nil {
		return nil, fmt.Errorf("decode environment state for seed telemetry: %w", err)
	}
	result := make([]core.Environment, 0, len(state.Environments))
	for name, environment := range state.Environments {
		if environment.Name == "" {
			environment.Name = name
		}
		if strings.TrimSpace(environment.Name) == "" || strings.TrimSpace(environment.RuntimeRef) == "" {
			return nil, fmt.Errorf("invalid Environment metadata while collecting seed telemetry: %w", core.ErrIncompatibleState)
		}
		result = append(result, environment)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func parseNerdctlImages(output string) ([]Image, error) {
	seen := map[string]struct{}{}
	images := make([]Image, 0)
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected nerdctl image row: %w", core.ErrIncompatibleState)
		}
		image := Image{
			Repository: strings.TrimSpace(parts[0]),
			Tag:        strings.TrimSpace(parts[1]),
			Digest:     strings.ToLower(strings.TrimSpace(parts[2])),
		}
		if image.Repository == "<none>" {
			continue
		}
		if image.Tag == "<none>" {
			image.Tag = ""
		}
		if image.Digest == "<none>" {
			image.Digest = ""
		}
		if err := validateImage(image); err != nil {
			return nil, err
		}
		key := image.Reference() + "@" + image.Digest
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		images = append(images, image)
	}
	sort.Slice(images, func(i, j int) bool {
		if images[i].Reference() != images[j].Reference() {
			return images[i].Reference() < images[j].Reference()
		}
		return images[i].Digest < images[j].Digest
	})
	return images, nil
}
