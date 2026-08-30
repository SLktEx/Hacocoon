package seedbuild

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

type ImageIdentity struct {
	Reference string `json:"reference"`
	Digest    string `json:"digest"`
}

func (i ImageIdentity) String() string { return i.Reference + "@" + i.Digest }

type BuildPlan struct {
	Parent          core.BaseRef
	ToolingRevision core.BaseRevision
	Images          []ImageIdentity
}

type BuildResult struct {
	Revision core.BaseRevision `json:"revision"`
	Alias    string            `json:"alias,omitempty"`
}

type Backend interface {
	ResolveParentBase(context.Context, core.BaseName) (core.BaseRef, error)
	BuildToolingBase(context.Context, core.BaseRef) (BuildResult, error)
	BuildSeed(context.Context, BuildPlan) (BuildResult, error)
}

type RecommendationSource interface {
	Driver() ociplugin.Driver
	SampleAll(context.Context, time.Duration) (ociplugin.SampleReport, error)
	Recommend(context.Context, time.Duration) ([]ociplugin.Recommendation, error)
}

type Service struct {
	backend Backend
	stats   RecommendationSource
	store   *Store
	now     func() time.Time
}

type BuildReport struct {
	Sampling          ociplugin.SampleReport `json:"sampling"`
	Parent            core.BaseRef           `json:"parent"`
	ToolingRevision   core.BaseRevision      `json:"tooling_revision"`
	SeedRevision      core.BaseRevision      `json:"seed_revision"`
	Images            []ImageIdentity        `json:"images"`
	ReusedToolingBase bool                   `json:"reused_tooling_base"`
	BuiltAt           time.Time              `json:"built_at"`
}

func New(backend Backend, stats RecommendationSource, store *Store) (*Service, error) {
	if backend == nil || stats == nil || store == nil {
		return nil, core.ErrInvalidArgument
	}
	return &Service{backend: backend, stats: stats, store: store, now: time.Now}, nil
}

func (s *Service) Build(ctx context.Context, base core.BaseName) (BuildReport, error) {
	if s == nil || s.backend == nil || s.stats == nil || s.store == nil {
		return BuildReport{}, core.ErrRuntimeUnavailable
	}
	// The physical Seed cache uses containerd namespaces. The optional Docker
	// driver intentionally does not authorize access to an arbitrary Host Docker
	// daemon, so v0.18 Seed publication currently requires the nerdctl plugin
	// driver and fails closed otherwise.
	if s.stats.Driver() != ociplugin.DriverNerdctl {
		return BuildReport{}, fmt.Errorf("OCI Seed build currently requires HACO_PLUGIN_OCI=nerdctl: %w", core.ErrUnsupported)
	}
	unlock, err := s.store.LockBuild()
	if err != nil {
		return BuildReport{}, err
	}
	defer unlock()

	parent, err := s.backend.ResolveParentBase(ctx, base)
	if err != nil {
		return BuildReport{}, err
	}

	sampling, err := s.stats.SampleAll(ctx, ociplugin.DefaultSampleMaxAge)
	if err != nil {
		return BuildReport{}, err
	}
	recommendations, err := s.stats.Recommend(ctx, ociplugin.DefaultRecommendationWindow)
	if err != nil {
		return BuildReport{}, err
	}
	images, err := selectedImages(recommendations)
	if err != nil {
		return BuildReport{}, err
	}

	toolingRevision, reused, err := s.ensureToolingBase(ctx, parent)
	if err != nil {
		return BuildReport{}, err
	}
	built, err := s.backend.BuildSeed(ctx, BuildPlan{
		Parent:          parent,
		ToolingRevision: toolingRevision,
		Images:          images,
	})
	if err != nil {
		return BuildReport{}, err
	}
	if err := validateRevision(built.Revision); err != nil {
		return BuildReport{}, fmt.Errorf("backend returned invalid Seed revision: %w", err)
	}

	manifest := Manifest{
		Parent:          parent,
		ToolingRevision: toolingRevision,
		SeedRevision:    built.Revision,
		SeedAlias:       built.Alias,
		Images:          images,
		BuiltAt:         s.now().UTC(),
	}
	if err := s.store.PutCurrent(ctx, manifest); err != nil {
		return BuildReport{}, errors.Join(
			fmt.Errorf("persist current Seed after successful publish: %w", err),
			core.ErrRecoveryRequired,
		)
	}
	return BuildReport{
		Sampling:          sampling,
		Parent:            parent,
		ToolingRevision:   toolingRevision,
		SeedRevision:      built.Revision,
		Images:            images,
		ReusedToolingBase: reused,
		BuiltAt:           manifest.BuiltAt,
	}, nil
}

func (s *Service) Current(ctx context.Context, base core.BaseName) (Manifest, error) {
	if s == nil || s.backend == nil || s.store == nil {
		return Manifest{}, core.ErrRuntimeUnavailable
	}
	parent, err := s.backend.ResolveParentBase(ctx, base)
	if err != nil {
		return Manifest{}, err
	}
	manifest, ok, err := s.store.CurrentManifest(ctx, parent)
	if err != nil {
		return Manifest{}, err
	}
	if !ok {
		return Manifest{}, fmt.Errorf("current Seed for Base %q: %w", parent.Name, core.ErrNotFound)
	}
	return manifest, nil
}

func (s *Service) ensureToolingBase(ctx context.Context, parent core.BaseRef) (core.BaseRevision, bool, error) {
	if manifest, ok, err := s.store.ToolingManifest(ctx, parent); err != nil {
		return "", false, err
	} else if ok {
		return manifest.ToolingRevision, true, nil
	}

	built, err := s.backend.BuildToolingBase(ctx, parent)
	if err != nil {
		return "", false, err
	}
	if err := validateRevision(built.Revision); err != nil {
		return "", false, fmt.Errorf("backend returned invalid tooling Base revision: %w", err)
	}
	manifest := ToolingManifest{
		Parent:          parent,
		ToolingRevision: built.Revision,
		ToolingAlias:    built.Alias,
		BuiltAt:         s.now().UTC(),
	}
	if err := s.store.PutTooling(ctx, manifest); err != nil {
		return "", false, errors.Join(
			fmt.Errorf("persist tooling Base after successful publish: %w", err),
			core.ErrRecoveryRequired,
		)
	}
	return built.Revision, false, nil
}

func selectedImages(recommendations []ociplugin.Recommendation) ([]ImageIdentity, error) {
	seen := map[string]struct{}{}
	result := make([]ImageIdentity, 0)
	for _, recommendation := range recommendations {
		if !recommendation.AutoPromote && !recommendation.Pinned {
			continue
		}
		image := ImageIdentity{Reference: recommendation.Reference, Digest: recommendation.Digest}
		if err := validateImageIdentity(image); err != nil {
			return nil, err
		}
		key := image.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, image)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Reference != result[j].Reference {
			return result[i].Reference < result[j].Reference
		}
		return result[i].Digest < result[j].Digest
	})
	return result, nil
}

func validateImageIdentity(image ImageIdentity) error {
	if strings.TrimSpace(image.Reference) == "" || strings.TrimSpace(image.Reference) != image.Reference || strings.ContainsAny(image.Reference, "@\t\r\n") {
		return fmt.Errorf("invalid Seed image reference %q: %w", image.Reference, core.ErrInvalidArgument)
	}
	return validateRevision(core.BaseRevision(image.Digest))
}

func validateRevision(revision core.BaseRevision) error {
	value := string(revision)
	if len(value) != len("sha256:")+64 || !strings.HasPrefix(value, "sha256:") {
		return fmt.Errorf("invalid immutable revision %q: %w", value, core.ErrIncompatibleState)
	}
	for _, r := range value[len("sha256:"):] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return fmt.Errorf("invalid immutable revision %q: %w", value, core.ErrIncompatibleState)
		}
	}
	return nil
}
