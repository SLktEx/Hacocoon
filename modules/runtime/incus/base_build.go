package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const builtBasePrefix = "hacocoon-base-"
const builtBaseDescription = "Hacocoon Base v1"

type baseAlias struct {
	Name        string `json:"name"`
	Target      string `json:"target"`
	Description string `json:"description"`
	Type        string `json:"type"`
}
type baseImage struct {
	Fingerprint string            `json:"fingerprint"`
	Public      bool              `json:"public"`
	Type        string            `json:"type"`
	Properties  map[string]string `json:"properties"`
}

func (p *BaseProvider) baseQuery(ctx context.Context, method, path string, data any, result any) error {
	args := []string{"query", "-X", method, path}
	if data != nil {
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		args = append(args, "--data", string(raw))
	}
	if method != "GET" {
		args = append(args, "--wait")
	}
	out, err := p.runner.Run(ctx, "incus", args...)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return fmt.Errorf("Incus Base %s %s failed: %w", method, path, core.ErrRuntimeUnavailable)
	}
	if result != nil && json.Unmarshal([]byte(out.Stdout), result) != nil {
		return core.ErrIncompatibleState
	}
	return nil
}
func (p *BaseProvider) basePath(path string) string {
	return "/1.0/images" + path + "?project=" + url.QueryEscape(p.project)
}
func (p *BaseProvider) baseAliases(ctx context.Context) ([]baseAlias, error) {
	var aliases []baseAlias
	if err := p.baseQuery(ctx, "GET", p.basePath("/aliases")+"&recursion=1", nil, &aliases); err != nil {
		return nil, err
	}
	if aliases == nil {
		return nil, core.ErrIncompatibleState
	}
	seen := map[string]bool{}
	for _, a := range aliases {
		if seen[a.Name] {
			return nil, core.ErrIncompatibleState
		}
		seen[a.Name] = true
	}
	return aliases, nil
}
func (p *BaseProvider) ownedBaseImage(ctx context.Context, name core.BaseName, a baseAlias) (baseImage, error) {
	var image baseImage
	if a.Description != builtBaseDescription || a.Type != "container" || !baseFingerprintPattern.MatchString(a.Target) {
		return image, core.ErrCapabilityStale
	}
	if err := p.baseQuery(ctx, "GET", p.basePath("/"+a.Target), nil, &image); err != nil {
		return image, err
	}
	if image.Fingerprint != a.Target || image.Type != "container" || image.Public || image.Properties["user.hacocoon.kind"] != "base-image" || image.Properties["user.hacocoon.base-name"] != string(name) || !core.ValidEnvironmentInstanceID(image.Properties["user.hacocoon.build-instance"]) {
		return image, core.ErrCapabilityStale
	}
	return image, nil
}
func (p *BaseProvider) builtBases(ctx context.Context) (map[core.BaseName]core.BaseInfo, error) {
	aliases, err := p.baseAliases(ctx)
	if err != nil {
		return nil, err
	}
	result := map[core.BaseName]core.BaseInfo{}
	for _, a := range aliases {
		if !strings.HasPrefix(a.Name, builtBasePrefix) {
			continue
		}
		name := core.BaseName(strings.TrimPrefix(a.Name, builtBasePrefix))
		if !basebuild.NamePattern.MatchString(string(name)) {
			return nil, core.ErrIncompatibleState
		}
		if _, exists := p.sources[name]; exists {
			return nil, core.ErrAlreadyExists
		}
		image, err := p.ownedBaseImage(ctx, name, a)
		if err != nil {
			return nil, err
		}
		result[name] = core.BaseInfo{Name: name, Revision: core.BaseRevision("sha256:" + image.Fingerprint)}
	}
	return result, nil
}

// PublishBase is called while the canonical temporary Env/Workspace locks are
// held. Incus stores the exact build identity with the image in its own database
// atomically at publication; no second Hacocoon image catalog is maintained.
func (p *BaseProvider) PublishBase(ctx context.Context, env core.Environment, lease core.WorkspaceLease, name core.BaseName) (core.BaseInfo, error) {
	result := core.BaseInfo{Name: name}
	if !basebuild.NamePattern.MatchString(string(name)) || !core.ValidTemporaryWorkspace(env.Workspace) || env.PersistentResource != (core.PersistentResourceRef{}) || lease.RuntimeRef != env.RuntimeRef || lease.EnvironmentID != env.Name || lease.WorkspaceID != env.Workspace.ID || lease.SourcePath != env.Workspace.Path || lease.State != core.WorkspaceLeaseActive || !core.ValidEnvironmentInstanceID(lease.InstanceID) {
		return result, core.ErrInvalidArgument
	}
	if _, exists := p.sources[name]; exists {
		return result, core.ErrAlreadyExists
	}
	if err := p.VerifyEnvironmentIdentity(ctx, env.RuntimeRef, lease.InstanceID); err != nil {
		return result, err
	}
	status, err := p.InspectEnvironment(ctx, env.RuntimeRef)
	if err != nil {
		return result, err
	}
	if status.State != core.EnvironmentStopped {
		return result, core.ErrIncompatibleState
	}
	// systemd may bind-mount machine-id read-only while running. Use Incus's
	// guest-file operation after positive stop; no Host path is traversed here.
	// Keep management operations on the decorated runner. The stdin decorator
	// deliberately accepts guest exec only; do not broaden that trust boundary.
	empty, err := p.runner.Run(ctx, "incus", "file", "push", "/dev/null", env.RuntimeRef+"/etc/machine-id", "--project", p.project, "--uid", "0", "--gid", "0", "--mode", "0644")
	if err != nil || empty.ExitCode != 0 {
		return result, fmt.Errorf("reset stopped builder machine-id: %w", core.ErrRuntimeUnavailable)
	}
	aliases, err := p.baseAliases(ctx)
	if err != nil {
		return result, err
	}
	aliasName := builtBasePrefix + string(name)
	var previous *baseAlias
	for _, a := range aliases {
		if a.Name == aliasName {
			if _, err := p.ownedBaseImage(ctx, name, a); err != nil {
				return result, err
			}
			copy := a
			previous = &copy
		}
	}
	// Clear instance templates: an image must not replay builder-specific files.
	// This is native metadata, not the guest filesystem or Host shell execution.
	metadataPath := "/1.0/instances/" + url.PathEscape(env.RuntimeRef) + "/metadata?project=" + url.QueryEscape(p.project)
	var metadata map[string]any
	if err := p.baseQuery(ctx, "GET", metadataPath, nil, &metadata); err != nil {
		return result, err
	}
	if metadata == nil {
		return result, core.ErrIncompatibleState
	}
	metadata["templates"] = map[string]any{}
	if err := p.baseQuery(ctx, "PUT", metadataPath, metadata, nil); err != nil {
		return result, err
	}
	buildAlias := "hacocoon-build-" + lease.InstanceID
	for _, a := range aliases {
		if a.Name == buildAlias {
			return result, core.ErrAlreadyExists
		}
	}
	properties := map[string]string{"user.hacocoon.kind": "base-image", "user.hacocoon.base-name": string(name), "user.hacocoon.build-instance": lease.InstanceID}
	data := map[string]any{"public": false, "auto_update": false, "properties": properties, "source": map[string]string{"type": "instance", "name": env.RuntimeRef}, "aliases": []map[string]string{{"name": buildAlias, "description": builtBaseDescription}}}
	if err := p.baseQuery(ctx, "POST", p.basePath(""), data, nil); err != nil {
		return result, fmt.Errorf("image publication unconfirmed; inspect Incus alias %s: %w", buildAlias, core.ErrRecoveryRequired)
	}
	// Image ownership already lives durably in Incus before this next fallible call.
	aliases, err = p.baseAliases(ctx)
	if err != nil {
		return result, err
	}
	var published *baseAlias
	for _, a := range aliases {
		if a.Name == buildAlias {
			copy := a
			published = &copy
		}
	}
	if published == nil {
		return result, core.ErrRecoveryRequired
	}
	image, err := p.ownedBaseImage(ctx, name, *published)
	if err != nil {
		return result, err
	}
	result.Revision = core.BaseRevision("sha256:" + image.Fingerprint)
	current := map[string]string{"name": aliasName, "target": image.Fingerprint, "description": builtBaseDescription}
	method, path := "POST", p.basePath("/aliases")
	if previous != nil {
		method = "PUT"
		path = p.basePath("/aliases/" + url.PathEscape(aliasName))
	}
	if err := p.baseQuery(ctx, method, path, current, nil); err != nil {
		return result, fmt.Errorf("image %s retained; Base alias update unconfirmed: %w", image.Fingerprint, core.ErrRecoveryRequired)
	}
	// Never delete an old revision or a newly published image on a pointer failure.
	return result, nil
}
