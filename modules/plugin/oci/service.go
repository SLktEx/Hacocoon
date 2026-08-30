package oci

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
	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	DefaultSampleMaxAge          = 6 * time.Hour
	DefaultRecommendationWindow = 30 * 24 * time.Hour
	DefaultAutoPromotionPercent = 10
	DefaultSeedNamespace        = "hacocoon-seed"
)

type Driver string

const (
	DriverNerdctl Driver = "nerdctl"
	DriverDocker  Driver = "docker"
)

func ParseDriver(value string) (Driver, error) {
	switch Driver(strings.ToLower(strings.TrimSpace(value))) {
	case DriverNerdctl:
		return DriverNerdctl, nil
	case DriverDocker:
		return DriverDocker, nil
	default:
		return "", fmt.Errorf("unsupported OCI plugin driver %q: %w", value, core.ErrInvalidArgument)
	}
}

type environmentExecutor interface {
	ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
}

type Service struct {
	runtime              environmentExecutor
	environmentStatePath string
	store                *Store
	driver               Driver
	hostRunner           host.Runner
	seedNamespace        string
	now                  func() time.Time
}

type Option func(*Service) error

func WithHostRunner(runner host.Runner) Option {
	return func(service *Service) error {
		if runner == nil {
			return core.ErrInvalidArgument
		}
		service.hostRunner = runner
		return nil
	}
}

func WithSeedNamespace(namespace string) Option {
	return func(service *Service) error {
		if strings.TrimSpace(namespace) == "" || strings.TrimSpace(namespace) != namespace || hasControl(namespace) {
			return core.ErrInvalidArgument
		}
		service.seedNamespace = namespace
		return nil
	}
}

type SampleReport struct {
	Sampled  int               `json:"sampled"`
	Fresh    int               `json:"fresh"`
	Failed   int               `json:"failed"`
	Failures map[string]string `json:"failures,omitempty"`
}

type Recommendation struct {
	Reference    string     `json:"reference"`
	Digest       string     `json:"digest"`
	Environments int        `json:"environments"`
	Percent      float64    `json:"percent"`
	LastSeen     time.Time  `json:"last_seen"`
	AutoPromote  bool       `json:"auto_promote"`
	Pinned       bool       `json:"pinned"`
	PinnedAt     *time.Time `json:"pinned_at,omitempty"`
	Reenabled    bool       `json:"re_enabled"`
}

func New(runtime environmentExecutor, environmentStatePath string, store *Store, driver Driver, options ...Option) (*Service, error) {
	if runtime == nil || strings.TrimSpace(environmentStatePath) == "" || store == nil {
		return nil, core.ErrInvalidArgument
	}
	if driver != DriverNerdctl && driver != DriverDocker {
		return nil, core.ErrInvalidArgument
	}
	service := &Service{
		runtime:              runtime,
		environmentStatePath: environmentStatePath,
		store:                store,
		driver:               driver,
		seedNamespace:        DefaultSeedNamespace,
		now:                  time.Now,
	}
	for _, option := range options {
		if option == nil {
			return nil, core.ErrInvalidArgument
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	return service, nil
}

func (s *Service) Driver() Driver {
	if s == nil {
		return ""
	}
	return s.driver
}

// SampleAll opportunistically records OCI image usage from Hacocoon-managed
// Environments. The concrete container CLI belongs to this optional plugin;
// Hacocoon Core does not require nerdctl, Docker, or containerd.
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

		images, sampleErr := s.listImages(ctx, environment.RuntimeRef)
		if sampleErr != nil {
			report.Failed++
			report.Failures[environment.Name] = sampleErr.Error()
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

func (s *Service) listImages(ctx context.Context, runtimeRef string) ([]Image, error) {
	result, execErr := s.runtime.ExecEnvironment(ctx, runtimeRef, core.ExecutionRequest{Argv: imageListArgv(s.driver)})
	if execErr != nil || result.ExitCode != 0 {
		return nil, commandFailure(fmt.Sprintf("%s image inventory", s.driver), result.Stderr, execErr, result.ExitCode)
	}
	return parseImageRows(result.Stdout, string(s.driver))
}

func (s *Service) Recommend(ctx context.Context, window time.Duration) ([]Recommendation, error) {
	if s == nil || s.store == nil || window <= 0 {
		return nil, core.ErrInvalidArgument
	}
	snapshots, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	policy, err := s.seedSelectionPolicy(ctx)
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
			if image.Digest == "" {
				continue
			}
			key := image.Reference() + "@" + image.Digest
			if policy.isBlocked(key) {
				continue
			}
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
	result := make([]Recommendation, 0, len(aggregates)+len(policy.pins))
	if denominator > 0 {
		for _, aggregate := range aggregates {
			key := aggregate.reference + "@" + aggregate.digest
			result = append(result, Recommendation{
				Reference:    aggregate.reference,
				Digest:       aggregate.digest,
				Environments: aggregate.count,
				Percent:      float64(aggregate.count) * 100 / float64(denominator),
				LastSeen:     aggregate.lastSeen,
				Reenabled:    policy.isReenabled(key),
			})
		}
	}
	sortRecommendations(result)
	markAutoPromotions(result, DefaultAutoPromotionPercent)
	result = applySeedPins(result, policy)
	sortRecommendations(result)
	return result, nil
}

func sortRecommendations(recommendations []Recommendation) {
	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].Environments != recommendations[j].Environments {
			return recommendations[i].Environments > recommendations[j].Environments
		}
		if !recommendations[i].LastSeen.Equal(recommendations[j].LastSeen) {
			return recommendations[i].LastSeen.After(recommendations[j].LastSeen)
		}
		if recommendations[i].Reference != recommendations[j].Reference {
			return recommendations[i].Reference < recommendations[j].Reference
		}
		return recommendations[i].Digest < recommendations[j].Digest
	})
}

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

func imageListArgv(driver Driver) []string {
	if driver == DriverDocker {
		return []string{"docker", "images", "--digests", "--format", "{{.Repository}}\t{{.Tag}}\t{{.Digest}}"}
	}
	return []string{"nerdctl", "images", "--format", "{{.Repository}}\t{{.Tag}}\t{{.Digest}}"}
}

func imageRemoveArgv(driver Driver, reference string) []string {
	if driver == DriverDocker {
		return []string{"docker", "image", "rm", reference}
	}
	return []string{"nerdctl", "rmi", reference}
}

func readEnvironments(path string) ([]core.Environment, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []core.Environment{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Environment state for OCI plugin: %w", err)
	}
	var state struct {
		Environments map[string]core.Environment `json:"environments"`
	}
	if err := json.Unmarshal(contents, &state); err != nil {
		return nil, fmt.Errorf("decode Environment state for OCI plugin: %w", err)
	}
	result := make([]core.Environment, 0, len(state.Environments))
	for name, environment := range state.Environments {
		if environment.Name == "" {
			environment.Name = name
		}
		if strings.TrimSpace(environment.Name) == "" || strings.TrimSpace(environment.RuntimeRef) == "" {
			return nil, fmt.Errorf("invalid Environment metadata while collecting OCI plugin telemetry: %w", core.ErrIncompatibleState)
		}
		result = append(result, environment)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func parseImageRows(output, source string) ([]Image, error) {
	seen := map[string]struct{}{}
	images := make([]Image, 0)
	for _, rawLine := range strings.Split(output, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected %s image row: %w", source, core.ErrIncompatibleState)
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
