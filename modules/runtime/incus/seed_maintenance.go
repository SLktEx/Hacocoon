package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/seedbuild"
)

var seedBuilderNamePattern = regexp.MustCompile(`^haco-(?:tooling|seed)-build-[0-9a-f]{12}$`)
var publishedSeedAliasPattern = regexp.MustCompile(`^hacocoon-(?:tooling|seed)-[a-z0-9][a-z0-9-]*-[0-9]+$`)

type seedMaintenanceInstance struct {
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}

type seedMaintenanceAlias struct {
	Name string `json:"name"`
}

type seedMaintenanceImage struct {
	Fingerprint string                 `json:"fingerprint"`
	Aliases     []seedMaintenanceAlias `json:"aliases"`
	UsedBy      []string               `json:"used_by"`
}

type seedMaintenancePlan struct {
	Builders []string
	Delete   []string
	Retain   map[string]string
}

func (p *SandboxProvider) MaintainSeedArtifacts(ctx context.Context, protection seedbuild.MaintenanceProtection, recoverBuilders bool) (seedbuild.MaintenanceReport, error) {
	if p == nil || p.BaseProvider == nil || p.Runtime == nil {
		return seedbuild.MaintenanceReport{}, core.ErrRuntimeUnavailable
	}
	if err := p.ensureProject(ctx); err != nil {
		return seedbuild.MaintenanceReport{}, err
	}

	instances, images, err := p.seedMaintenanceInventory(ctx)
	if err != nil {
		return seedbuild.MaintenanceReport{}, err
	}
	plan, err := planSeedMaintenance(instances, images, protection, recoverBuilders)
	if err != nil {
		return seedbuild.MaintenanceReport{}, err
	}
	report := seedbuild.MaintenanceReport{RetainedImages: map[string]string{}, Failures: map[string]string{}}

	if recoverBuilders {
		for _, builder := range plan.Builders {
			if _, err := p.runner.Run(ctx, "incus", "delete", builder, "--project", p.project, "--force"); err != nil {
				report.Failures["builder:"+builder] = err.Error()
				continue
			}
			report.DeletedBuilders = append(report.DeletedBuilders, builder)
		}
		// Re-read both inventories after cleanup. A builder contributes a
		// volatile.base_image dependency while it exists, and a concurrent Host
		// operator may also have changed project state between the two calls.
		instances, images, err = p.seedMaintenanceInventory(ctx)
		if err != nil {
			return report, errors.Join(err, core.ErrRecoveryRequired)
		}
		plan, err = planSeedMaintenance(instances, images, protection, false)
		if err != nil {
			return report, errors.Join(err, core.ErrRecoveryRequired)
		}
	}
	for revision, reason := range plan.Retain {
		report.RetainedImages[revision] = reason
	}
	for _, fingerprint := range plan.Delete {
		revision := core.BaseRevision("sha256:" + fingerprint)
		if _, err := p.runner.Run(ctx, "incus", "image", "delete", fingerprint, "--project", p.project); err != nil {
			report.Failures[string(revision)] = err.Error()
			continue
		}
		report.DeletedImages = append(report.DeletedImages, revision)
	}

	sort.Strings(report.DeletedBuilders)
	sort.Slice(report.DeletedImages, func(i, j int) bool { return report.DeletedImages[i] < report.DeletedImages[j] })
	if len(report.RetainedImages) == 0 {
		report.RetainedImages = nil
	}
	if len(report.Failures) == 0 {
		report.Failures = nil
		return report, nil
	}
	return report, errors.Join(fmt.Errorf("Incus Seed maintenance completed only partially"), core.ErrRecoveryRequired)
}

func (p *SandboxProvider) seedMaintenanceInventory(ctx context.Context) ([]seedMaintenanceInstance, []seedMaintenanceImage, error) {
	instanceResult, err := p.runner.Run(ctx, "incus", "list", "--project", p.project, "--format", "json")
	if err != nil {
		return nil, nil, fmt.Errorf("list Incus project instances for Seed maintenance: %w", err)
	}
	var instances []seedMaintenanceInstance
	if err := json.Unmarshal([]byte(instanceResult.Stdout), &instances); err != nil {
		return nil, nil, fmt.Errorf("decode Incus project instances for Seed maintenance: %w", core.ErrIncompatibleState)
	}

	imageResult, err := p.runner.Run(ctx, "incus", "image", "list", "--project", p.project, "--format", "json")
	if err != nil {
		return nil, nil, fmt.Errorf("list Incus project images for Seed maintenance: %w", err)
	}
	var images []seedMaintenanceImage
	if err := json.Unmarshal([]byte(imageResult.Stdout), &images); err != nil {
		return nil, nil, fmt.Errorf("decode Incus project images for Seed maintenance: %w", core.ErrIncompatibleState)
	}
	return instances, images, nil
}

func planSeedMaintenance(instances []seedMaintenanceInstance, images []seedMaintenanceImage, protection seedbuild.MaintenanceProtection, recoverBuilders bool) (seedMaintenancePlan, error) {
	protected := map[string]struct{}{}
	for _, revision := range protection.Revisions {
		fingerprint, err := seedFingerprint(revision)
		if err != nil {
			return seedMaintenancePlan{}, fmt.Errorf("invalid protected Seed revision %q: %w", revision, core.ErrIncompatibleState)
		}
		protected[fingerprint] = struct{}{}
	}
	protectedAliases := map[string]struct{}{}
	for _, alias := range protection.Aliases {
		if !publishedSeedAliasPattern.MatchString(alias) {
			return seedMaintenancePlan{}, fmt.Errorf("invalid protected Seed alias %q: %w", alias, core.ErrIncompatibleState)
		}
		protectedAliases[alias] = struct{}{}
	}

	instanceBases := map[string]struct{}{}
	builders := make([]string, 0)
	for _, instance := range instances {
		if recoverBuilders && seedBuilderNamePattern.MatchString(instance.Name) {
			builders = append(builders, instance.Name)
		}
		base := strings.ToLower(strings.TrimSpace(instance.Config["volatile.base_image"]))
		if base == "" {
			continue
		}
		if !baseFingerprintPattern.MatchString(base) {
			return seedMaintenancePlan{}, fmt.Errorf("Incus instance %q has invalid volatile.base_image %q: %w", instance.Name, base, core.ErrIncompatibleState)
		}
		instanceBases[base] = struct{}{}
	}

	plan := seedMaintenancePlan{Builders: builders, Retain: map[string]string{}}
	seenFingerprint := map[string]struct{}{}
	for _, image := range images {
		fingerprint := strings.ToLower(strings.TrimSpace(image.Fingerprint))
		if !baseFingerprintPattern.MatchString(fingerprint) {
			return seedMaintenancePlan{}, fmt.Errorf("Incus image returned invalid fingerprint %q: %w", image.Fingerprint, core.ErrIncompatibleState)
		}
		if _, duplicate := seenFingerprint[fingerprint]; duplicate {
			return seedMaintenancePlan{}, fmt.Errorf("Incus image inventory repeats fingerprint %s: %w", fingerprint, core.ErrIncompatibleState)
		}
		seenFingerprint[fingerprint] = struct{}{}

		ownedAliases := make([]string, 0)
		hasExternalAlias := false
		for _, alias := range image.Aliases {
			name := strings.TrimSpace(alias.Name)
			if publishedSeedAliasPattern.MatchString(name) {
				ownedAliases = append(ownedAliases, name)
				continue
			}
			if name != "" {
				hasExternalAlias = true
			}
		}
		if len(ownedAliases) == 0 {
			continue
		}
		revision := "sha256:" + fingerprint
		if _, ok := protected[fingerprint]; ok {
			plan.Retain[revision] = "current-manifest"
			continue
		}
		aliasProtected := false
		for _, alias := range ownedAliases {
			if _, ok := protectedAliases[alias]; ok {
				aliasProtected = true
				break
			}
		}
		if aliasProtected {
			plan.Retain[revision] = "current-alias"
			continue
		}
		if hasExternalAlias {
			plan.Retain[revision] = "external-alias"
			continue
		}
		if _, ok := instanceBases[fingerprint]; ok {
			plan.Retain[revision] = "instance-base"
			continue
		}
		if len(image.UsedBy) > 0 {
			plan.Retain[revision] = "incus-used-by"
			continue
		}
		plan.Delete = append(plan.Delete, fingerprint)
	}
	sort.Strings(plan.Builders)
	sort.Strings(plan.Delete)
	return plan, nil
}

var _ seedbuild.MaintenanceBackend = (*SandboxProvider)(nil)
