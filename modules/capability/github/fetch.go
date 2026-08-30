package gitcap

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

type FetchSpec struct {
	Environment string
	Remote      string
}

// Fetch updates the registered workspace's remote-tracking refs without ever
// copying host credentials into the Environment. Authentication is performed
// by the same isolated host Git runner used for push.
func (b *Broker) Fetch(ctx context.Context, spec FetchSpec) (core.CapabilityResult, error) {
	if b == nil || b.runner == nil || b.store == nil || b.capabilities == nil {
		return core.CapabilityResult{}, fmt.Errorf("git capability is unavailable: %w", core.ErrUnsupported)
	}
	spec.Environment = strings.TrimSpace(spec.Environment)
	spec.Remote = strings.TrimSpace(spec.Remote)
	if spec.Remote == "" {
		spec.Remote = "origin"
	}
	if spec.Environment == "" || !safeRemoteName(spec.Remote) {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}

	environment, err := b.store.GetEnvironment(ctx, spec.Environment)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	boundary, err := resolveRepositoryBoundary(environment.Workspace.Path)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	runner := pinGitRepository(b.runner, boundary)
	if err := rejectUnsafeLocalGitConfig(ctx, runner, environment.Workspace.Path, spec.Remote); err != nil {
		return core.CapabilityResult{}, err
	}
	remoteURL, repository, err := resolveGitHubRemote(ctx, runner, environment.Workspace.Path, spec.Remote)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	resource := repository.Resource("fetch/" + spec.Remote)

	return b.capabilities.Request(ctx, core.CapabilityRequest{
		Capability:  GitHubCapability,
		Action:      "fetch",
		Resource:    resource,
		Environment: environment.Name,
		Attributes: map[string]string{
			"organization":        repository.Owner,
			"repository":          repository.Name,
			"repository_identity": boundary.Identity,
			"remote":              spec.Remote,
		},
		Parameters: map[string]string{"remote_url": remoteURL},
	})
}

// UnifiedProvider extends the existing push provider with brokered fetch while
// keeping one github.git capability registration in the capability service.
type UnifiedProvider struct {
	push   *Provider
	runner host.Runner
	store  EnvironmentStore
}

func NewUnifiedProvider(runner host.Runner, store EnvironmentStore) *UnifiedProvider {
	return &UnifiedProvider{
		push:   NewProvider(runner, store),
		runner: newIsolatedGitRunner(runner),
		store:  store,
	}
}

func (*UnifiedProvider) Capability() string { return GitHubCapability }

// Preserve the existing provider's explicit contract: remote_url is transport
// compatibility metadata only. Authority is re-derived from the pinned
// workspace and policy-visible Attributes before either fetch or push runs.
func (*UnifiedProvider) NonAuthorityParameters() []string {
	return []string{"remote_url"}
}

func (p *UnifiedProvider) Execute(ctx context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	if req.Action != "fetch" {
		if p == nil || p.push == nil {
			return core.CapabilityResult{}, core.ErrUnsupported
		}
		return p.push.Execute(ctx, req)
	}
	if req.Capability != GitHubCapability || p == nil || p.runner == nil || p.store == nil {
		return core.CapabilityResult{}, core.ErrUnsupported
	}

	environment, err := p.store.GetEnvironment(ctx, req.Environment)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	remote := strings.TrimSpace(req.Attributes["remote"])
	if !safeRemoteName(remote) {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	boundary, err := resolveRepositoryBoundary(environment.Workspace.Path)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if approvedIdentity := req.Attributes["repository_identity"]; approvedIdentity == "" || approvedIdentity != boundary.Identity {
		return core.CapabilityResult{}, fmt.Errorf("Git repository identity changed after policy evaluation: %w", core.ErrCapabilityStale)
	}

	runner := pinGitRepository(p.runner, boundary)
	if err := rejectUnsafeLocalGitConfig(ctx, runner, environment.Workspace.Path, remote); err != nil {
		return core.CapabilityResult{}, err
	}
	remoteURL, repository, err := resolveGitHubRemote(ctx, runner, environment.Workspace.Path, remote)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if req.Resource != repository.Resource("fetch/"+remote) || req.Attributes["organization"] != repository.Owner || req.Attributes["repository"] != repository.Name {
		return core.CapabilityResult{}, fmt.Errorf("git remote changed after policy evaluation: %w", core.ErrCapabilityStale)
	}
	if approvedURL := strings.TrimSpace(req.Parameters["remote_url"]); approvedURL == "" || approvedURL != remoteURL {
		return core.CapabilityResult{}, fmt.Errorf("git remote URL changed after policy evaluation: %w", core.ErrCapabilityStale)
	}

	// Execute against the already validated URL rather than the remote name so
	// repository-controlled remote.<name>.fetch refspecs cannot redirect writes
	// into arbitrary local refs. Submodules and tags are intentionally excluded;
	// the broker only updates branch remote-tracking refs.
	refspec := "+refs/heads/*:refs/remotes/" + remote + "/*"
	args := []string{
		"-c", "core.hooksPath=/dev/null",
		"-C", environment.Workspace.Path,
		"fetch",
		"--prune",
		"--no-tags",
		"--no-recurse-submodules",
		remoteURL,
		refspec,
	}
	result, err := runner.Run(ctx, "git", args...)
	if err != nil {
		return core.CapabilityResult{}, fmt.Errorf("brokered git fetch failed: %w%s", err, sanitizedGitDetail(result.Stderr))
	}
	return core.CapabilityResult{Provider: GitHubCapability, Output: strings.TrimSpace(result.Stdout)}, nil
}
