// Package vscode implements the optional editor adapter, never Core authority.
package vscode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/experimental"
	"github.com/blang/semver/v4"
)

const galleryURL = "https://marketplace.visualstudio.com/_apis/public/gallery/extensionquery"
const maxGalleryBytes = 16 << 20

type Release struct {
	Version        string `json:"version"`
	LastUpdated    string `json:"lastUpdated"`
	TargetPlatform string `json:"targetPlatform"`
	Properties     []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"properties"`
}

func (r Release) preRelease() (bool, error) {
	pre, seen := false, false
	for _, p := range r.Properties {
		if p.Key == "Microsoft.VisualStudio.Code.PreRelease" {
			if seen || (p.Value != "true" && p.Value != "false") {
				return false, fmt.Errorf("invalid pre-release metadata")
			}
			pre, seen = p.Value == "true", true
		}
	}
	return pre, nil
}

// Select chooses by semantic version, not response order or publication order.
// Exact pins bypass age/pre-release policy, but never platform/metadata validation.
func Select(releases []Release, e experimental.Extension, policy experimental.Extensions, platform string, now time.Time) (Release, error) {
	v := experimental.VSCode{Extensions: policy}
	v.Extensions.Install = []experimental.Extension{e}
	if err := v.Validate(); err != nil {
		return Release{}, err
	}
	if platform != "linux-x64" && platform != "linux-arm64" {
		return Release{}, fmt.Errorf("unsupported VS Code platform")
	}
	age, _ := experimental.ReleaseAge(policy.MinReleaseAge)
	prePolicy := e.PreRelease
	if prePolicy == "" {
		prePolicy = policy.PreRelease
	}
	var best Release
	var bestVersion semver.Version
	seen := map[string]bool{}
	hasTarget := map[string]bool{}
	for _, r := range releases {
		if r.TargetPlatform == platform {
			hasTarget[r.Version] = true
		}
	}
	for _, r := range releases {
		if r.TargetPlatform != "" && r.TargetPlatform != "universal" && r.TargetPlatform != platform {
			continue
		}
		// The VS Code CLI prefers a platform build over the universal artifact.
		// Do not age-check an older universal artifact and install a newer build.
		if hasTarget[r.Version] && r.TargetPlatform != platform {
			continue
		}
		version, err := semver.Parse(r.Version)
		if err != nil || len(r.Version) > 128 {
			return Release{}, fmt.Errorf("invalid gallery version")
		}
		key := r.Version + "/" + r.TargetPlatform
		if seen[key] {
			return Release{}, fmt.Errorf("duplicate gallery release")
		}
		seen[key] = true
		pre, err := r.preRelease()
		if err != nil {
			return Release{}, err
		}
		date, err := time.Parse(time.RFC3339Nano, r.LastUpdated)
		if err != nil || date.IsZero() || date.After(now) {
			return Release{}, fmt.Errorf("invalid gallery release date")
		}
		if e.Version != "" {
			if r.Version != e.Version {
				continue
			}
		} else {
			if date.After(now.Add(-age)) || (prePolicy != "allow" && (pre || len(version.Pre) > 0)) {
				continue
			}
		}
		if best.Version == "" || version.GT(bestVersion) || (version.EQ(bestVersion) && r.TargetPlatform == platform) {
			best, bestVersion = r, version
		}
	}
	if best.Version == "" {
		return Release{}, fmt.Errorf("no extension release satisfies version, age, pre-release and platform constraints")
	}
	return best, nil
}

type Gallery struct{ Client *http.Client }

func (g Gallery) Releases(ctx context.Context, id string) ([]Release, error) {
	if !experimental.ValidID(id) {
		return nil, fmt.Errorf("invalid extension id")
	}
	// IncludeVersions (1), IncludeVersionProperties (16). Never latest-only.
	body, _ := json.Marshal(map[string]any{"filters": []any{map[string]any{"criteria": []any{map[string]any{"filterType": 7, "value": id}}, "pageNumber": 1, "pageSize": 1}}, "flags": 17})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, galleryURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json;api-version=7.2-preview.1")
	client := g.Client
	if client == nil {
		client = &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("extension gallery request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("extension gallery returned HTTP %d", response.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, maxGalleryBytes+1))
	if err != nil || len(b) > maxGalleryBytes {
		return nil, fmt.Errorf("invalid gallery response size")
	}
	var payload struct {
		Results []struct {
			Extensions []struct {
				ExtensionName string `json:"extensionName"`
				Publisher     struct {
					PublisherName string `json:"publisherName"`
				} `json:"publisher"`
				Versions []Release `json:"versions"`
			} `json:"extensions"`
		} `json:"results"`
	}
	if json.Unmarshal(b, &payload) != nil || len(payload.Results) != 1 || len(payload.Results[0].Extensions) != 1 {
		return nil, fmt.Errorf("invalid or missing gallery extension")
	}
	e := payload.Results[0].Extensions[0]
	if !strings.EqualFold(e.Publisher.PublisherName+"."+e.ExtensionName, id) || len(e.Versions) == 0 {
		return nil, fmt.Errorf("gallery identity mismatch or missing releases")
	}
	return e.Versions, nil
}

type Install struct {
	ID         string `json:"id"`
	Version    string `json:"version"`
	PreRelease bool   `json:"preRelease,omitempty"`
}

// Resolve includes dependencies and extension packs under the same policy.
// The server CLI is told not to resolve extra unpinned dependencies itself.
func Resolve(ctx context.Context, g Gallery, cfg experimental.VSCode, platform string, now time.Time) ([]Install, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	explicit := map[string]experimental.Extension{}
	for _, e := range cfg.Extensions.Install {
		explicit[strings.ToLower(e.ID)] = e
	}
	visited := map[string]bool{}
	var result []Install
	var visit func(experimental.Extension) error
	visit = func(e experimental.Extension) error {
		e.ID = strings.ToLower(e.ID)
		if visited[e.ID] {
			return nil
		}
		visited[e.ID] = true
		if len(visited) > 256 {
			return fmt.Errorf("extension dependency limit exceeded")
		}
		if override, ok := explicit[e.ID]; ok {
			e = override
			e.ID = strings.ToLower(e.ID)
		}
		releases, err := g.Releases(ctx, e.ID)
		if err != nil {
			return err
		}
		r, err := Select(releases, e, cfg.Extensions, platform, now)
		if err != nil {
			return fmt.Errorf("select %s: %w", e.ID, err)
		}
		for _, p := range r.Properties {
			if p.Key == "Microsoft.VisualStudio.Code.ExtensionDependencies" || p.Key == "Microsoft.VisualStudio.Code.ExtensionPack" {
				if p.Value == "" {
					continue
				}
				for _, id := range strings.Split(p.Value, ",") {
					if !experimental.ValidID(id) {
						return fmt.Errorf("invalid gallery dependency")
					}
					if err := visit(experimental.Extension{ID: id}); err != nil {
						return err
					}
				}
			}
		}
		pre, _ := r.preRelease()
		result = append(result, Install{ID: e.ID, Version: r.Version, PreRelease: pre})
		return nil
	}
	for _, e := range cfg.Extensions.Install {
		if err := visit(e); err != nil {
			return nil, err
		}
	}
	return result, nil
}
