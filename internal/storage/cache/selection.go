// Package cache owns trusted Host cache selection, not provider mounts or data
// lifecycle. Ordinary Environments produce cache contents; Core owns copies.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"path"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/git"
)

const Kind = "build-cache"

// Area is trusted configuration. Repository paths are relative to that named
// repository; paths without a repository are absolute guest paths, never Host
// paths. Compatibility is an explicit Host-owned tool/platform format identity.
type Area struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Repository    string `json:"repository,omitempty"`
	Compatibility string `json:"compatibility"`
	Scope         string `json:"scope,omitempty"`
	Group         string `json:"group,omitempty"`
}

type Configuration struct {
	Areas []Area `json:"areas"`
}

type Generations interface {
	EnsureResourceGeneration(context.Context, string, string, string) (core.ResourceGeneration, error)
}

type Repositories interface {
	Get(string, string) (gitrepo.Object, error)
}

// Selector is immutable after construction. The configuration must come from
// the trusted Host, never an Environment file or a client mount request.
type Selector struct {
	areas        []Area
	generations  Generations
	repositories Repositories
}

func NewSelector(configuration Configuration, generations Generations, repositories Repositories) (*Selector, error) {
	if generations == nil {
		return nil, core.ErrInvalidArgument
	}
	areas, err := validateConfiguration(configuration)
	if err != nil {
		return nil, err
	}
	return &Selector{areas: areas, generations: generations, repositories: repositories}, nil
}

func validateConfiguration(configuration Configuration) ([]Area, error) {
	if len(configuration.Areas) > core.MaxEnvironmentAttachments {
		return nil, core.ErrInvalidArgument
	}
	areas := slices.Clone(configuration.Areas)
	slices.SortFunc(areas, func(a, b Area) int { return strings.Compare(a.Name, b.Name) })
	for i := range areas {
		a := &areas[i]
		if a.Scope == "" {
			a.Scope = "workspace"
		}
		if !core.ValidResourceGenerationSpec(a.Name, Kind, strings.Repeat("0", 64)) || i > 0 && areas[i-1].Name == a.Name || !cleanText(a.Compatibility, 256) {
			return nil, core.ErrInvalidArgument
		}
		if a.Scope != "workspace" && a.Scope != "shared" || a.Scope == "workspace" && a.Group != "" || a.Scope == "shared" && !cleanText(a.Group, 128) {
			return nil, core.ErrInvalidArgument
		}
		if !cleanPath(a.Path) || a.Repository == "" && !strings.HasPrefix(a.Path, "/") || a.Repository != "" && (strings.HasPrefix(a.Path, "/") || a.Path == ".." || strings.HasPrefix(a.Path, "../") || !gitadapter.ValidID(a.Repository)) {
			return nil, core.ErrInvalidArgument
		}
		// Repository destinations must be named explicitly. An absolute workspace
		// path would bypass the rule's repository applicability and member binding.
		if a.Repository == "" && (a.Path == "/workspace" || strings.HasPrefix(a.Path, "/workspace/")) {
			return nil, core.ErrInvalidArgument
		}
	}
	return areas, nil
}

func cleanText(value string, limit int) bool {
	return value != "" && len(value) <= limit && utf8.ValidString(value) && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}

func cleanPath(value string) bool {
	return cleanText(value, 1024) && value != "/" && value != "." && path.Clean(value) == value && !strings.Contains(value, "\\")
}

type selectionPlan struct {
	area   Area
	target string
	name   string
	digest string
}

// Select first resolves and validates the whole request before initializing any
// generations. It never adopts an existing Environment or mutates cache data.
func (s *Selector) Select(ctx context.Context, request core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
	if s == nil || !core.ValidEnvironmentInstanceID(request.InstanceID) || request.Workspace.ID == "" {
		return nil, core.ErrInvalidArgument
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.ReadOnly || len(s.areas) == 0 {
		return nil, nil
	}
	var members map[string]string
	for _, area := range s.areas {
		if area.Repository != "" {
			var err error
			members, err = s.repositoryPaths(request.Workspace)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	plans := make([]selectionPlan, 0, len(s.areas))
	for _, area := range s.areas {
		target := area.Path
		if area.Repository != "" {
			base, found := members[area.Repository]
			if !found {
				continue // This rule does not apply to the selected Workspace.
			}
			target = base + "/" + area.Path
		}
		if !cleanPath(target) {
			return nil, core.ErrInvalidArgument
		}
		for _, previous := range plans {
			if previous.target == target || strings.HasPrefix(target, previous.target+"/") || strings.HasPrefix(previous.target, target+"/") {
				return nil, fmt.Errorf("cache destinations overlap: %w", core.ErrInvalidArgument)
			}
		}
		// Structured encoding separates fields unambiguously. Base and Env names
		// are intentionally absent; compatibility is configured, not inferred.
		scope := area.Group
		if area.Scope == "workspace" {
			scope = string(request.Workspace.ID)
		}
		data, err := json.Marshal(struct {
			Contract string
			Area     Area
			Scope    string
		}{"hacocoon-cache-selection-1", area, scope})
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(data)
		encoded := hex.EncodeToString(digest[:])
		// The catalog validates the full digest even if a shortened name collides.
		plans = append(plans, selectionPlan{area, target, "cache-" + encoded[:34], encoded})
	}
	result := make([]core.EnvironmentResourceSelection, 0, len(plans))
	for _, plan := range plans {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		generation, err := s.generations.EnsureResourceGeneration(ctx, plan.name, Kind, plan.digest)
		if err != nil {
			return nil, err
		}
		if !core.ValidResourceGeneration(generation) || generation.Name != plan.name || generation.Kind != Kind || generation.Compatibility != plan.digest {
			return nil, core.ErrIncompatibleState
		}
		result = append(result, core.EnvironmentResourceSelection{Key: plan.area.Name, Target: plan.target, Origin: generation})
	}
	return result, nil
}

func (s *Selector) repositoryPaths(work core.Workspace) (map[string]string, error) {
	id, managed := strings.CutPrefix(work.Path, "managed:")
	if !managed {
		return nil, nil // External Workspaces are not enrolled as repository data.
	}
	if !gitadapter.ValidID(id) || s.repositories == nil {
		return nil, core.ErrUnsupported
	}
	object, err := s.repositories.Get("work", id)
	if err != nil {
		return nil, err
	}
	if object.ID != id || object.Kind != "work" || object.State != "ready" || len(object.Members) > 8 || core.WorkspaceID("workspace:managed:"+object.Owner) != work.ID {
		return nil, core.ErrCapabilityStale
	}
	owner, err := hex.DecodeString(object.Owner)
	if err != nil || len(owner) != 16 {
		return nil, core.ErrIncompatibleState
	}
	paths := map[string]string{}
	for _, member := range object.Copies() {
		if !gitadapter.ValidID(member.Repository) || paths[member.Repository] != "" || member.State != "ready" || member.Kind != "work" {
			return nil, core.ErrIncompatibleState
		}
		target := "/workspace"
		if len(object.Members) != 0 {
			target += "/" + member.Repository
		}
		paths[member.Repository] = target
	}
	return paths, nil
}
