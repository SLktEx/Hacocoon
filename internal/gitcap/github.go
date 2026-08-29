package gitcap

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const GitHubCapability = "github.git"

var (
	remoteNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	slugPartPattern   = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
)

type EnvironmentStore interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
}

type PushSpec struct {
	Environment string
	Remote      string
	Source      string
	Branch      string
	Force       bool
}

type Broker struct {
	runner       host.Runner
	store        EnvironmentStore
	capabilities *capabilityapp.Service
}

func NewBroker(runner host.Runner, store EnvironmentStore, capabilities *capabilityapp.Service) *Broker {
	return &Broker{runner: runner, store: store, capabilities: capabilities}
}

func (b *Broker) Push(ctx context.Context, spec PushSpec) (core.CapabilityResult, error) {
	if b == nil || b.runner == nil || b.store == nil || b.capabilities == nil {
		return core.CapabilityResult{}, fmt.Errorf("git capability is unavailable: %w", core.ErrUnsupported)
	}
	spec.Environment = strings.TrimSpace(spec.Environment)
	spec.Remote = strings.TrimSpace(spec.Remote)
	spec.Source = strings.TrimSpace(spec.Source)
	spec.Branch = strings.TrimSpace(spec.Branch)
	if spec.Remote == "" {
		spec.Remote = "origin"
	}
	if spec.Source == "" {
		spec.Source = "HEAD"
	}
	if spec.Environment == "" || !safeRemoteName(spec.Remote) || !safeRevision(spec.Source) {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	environment, err := b.store.GetEnvironment(ctx, spec.Environment)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if err := rejectUnsafeLocalGitConfig(ctx, b.runner, environment.Workspace.Path, spec.Remote); err != nil {
		return core.CapabilityResult{}, err
	}
	targetRef, err := normalizeBranch(ctx, b.runner, environment.Workspace.Path, spec.Branch)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	remoteURL, repository, err := resolveGitHubRemote(ctx, b.runner, environment.Workspace.Path, spec.Remote)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	sourceSHA, err := resolveCommit(ctx, b.runner, environment.Workspace.Path, spec.Source)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	action := "push"
	attributes := map[string]string{
		"organization": repository.Owner,
		"repository":   repository.Name,
		"remote":       spec.Remote,
		"source_sha":   sourceSHA,
		"target_ref":   targetRef,
	}
	if spec.Force {
		action = "force-push"
		expected, err := resolveRemoteRef(ctx, b.runner, environment.Workspace.Path, remoteURL, targetRef)
		if err != nil {
			return core.CapabilityResult{}, err
		}
		if expected == "" {
			return core.CapabilityResult{}, fmt.Errorf("force target %s does not exist: %w", targetRef, core.ErrInvalidArgument)
		}
		attributes["expected_remote_sha"] = expected
	}
	return b.capabilities.Request(ctx, core.CapabilityRequest{
		Capability:  GitHubCapability,
		Action:      action,
		Resource:    repository.Resource(targetRef),
		Environment: environment.Name,
		Attributes:  attributes,
		Parameters:  map[string]string{"remote_url": remoteURL},
	})
}

type Provider struct {
	runner host.Runner
	store  EnvironmentStore
}

func NewProvider(runner host.Runner, store EnvironmentStore) *Provider {
	return &Provider{runner: runner, store: store}
}

func (*Provider) Capability() string { return GitHubCapability }

func (p *Provider) Execute(ctx context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	if req.Capability != GitHubCapability || (req.Action != "push" && req.Action != "force-push") {
		return core.CapabilityResult{}, core.ErrUnsupported
	}
	if p == nil || p.runner == nil || p.store == nil {
		return core.CapabilityResult{}, core.ErrUnsupported
	}
	environment, err := p.store.GetEnvironment(ctx, req.Environment)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	remote := req.Attributes["remote"]
	targetRef := req.Attributes["target_ref"]
	sourceSHA := strings.ToLower(req.Attributes["source_sha"])
	if !safeRemoteName(remote) || !validTargetRef(targetRef) || !validObjectID(sourceSHA) {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	if err := rejectUnsafeLocalGitConfig(ctx, p.runner, environment.Workspace.Path, remote); err != nil {
		return core.CapabilityResult{}, err
	}
	remoteURL, repository, err := resolveGitHubRemote(ctx, p.runner, environment.Workspace.Path, remote)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if req.Resource != repository.Resource(targetRef) || req.Attributes["organization"] != repository.Owner || req.Attributes["repository"] != repository.Name {
		return core.CapabilityResult{}, fmt.Errorf("git remote or target changed after policy evaluation: %w", core.ErrCapabilityStale)
	}
	if approvedURL := strings.TrimSpace(req.Parameters["remote_url"]); approvedURL == "" || approvedURL != remoteURL {
		return core.CapabilityResult{}, fmt.Errorf("git remote URL changed after policy evaluation: %w", core.ErrCapabilityStale)
	}
	if err := ensureCommit(ctx, p.runner, environment.Workspace.Path, sourceSHA); err != nil {
		return core.CapabilityResult{}, err
	}
	args := []string{"-c", "core.hooksPath=/dev/null", "-C", environment.Workspace.Path, "push", "--porcelain", "--no-verify"}
	if req.Action == "force-push" {
		expected := strings.ToLower(req.Attributes["expected_remote_sha"])
		if !validObjectID(expected) {
			return core.CapabilityResult{}, core.ErrInvalidArgument
		}
		current, err := resolveRemoteRef(ctx, p.runner, environment.Workspace.Path, remoteURL, targetRef)
		if err != nil {
			return core.CapabilityResult{}, err
		}
		if strings.ToLower(current) != expected {
			return core.CapabilityResult{}, fmt.Errorf("remote ref changed after approval: %w", core.ErrCapabilityStale)
		}
		args = append(args, "--force-with-lease="+targetRef+":"+expected)
	}
	// Push the exact URL that was re-resolved and compared after policy evaluation.
	// Never pass the remote name here: Git gives remote.<name>.pushurl authority over
	// the actual destination, which would turn a repository-local config value into
	// an authorization bypass.
	args = append(args, remoteURL, sourceSHA+":"+targetRef)
	result, err := p.runner.Run(ctx, "git", args...)
	if err != nil {
		return core.CapabilityResult{}, fmt.Errorf("brokered git push failed: %w%s", err, sanitizedGitDetail(result.Stderr))
	}
	return core.CapabilityResult{Provider: GitHubCapability, Output: strings.TrimSpace(result.Stdout)}, nil
}

type GitHubRepository struct {
	Owner string
	Name  string
}

func (r GitHubRepository) Resource(targetRef string) string {
	return "github://" + r.Owner + "/" + r.Name + "/" + targetRef
}

func resolveGitHubRemote(ctx context.Context, runner host.Runner, workspace, remote string) (string, GitHubRepository, error) {
	result, err := runner.Run(ctx, "git", "-C", workspace, "config", "--get", "remote."+remote+".url")
	if err != nil {
		return "", GitHubRepository{}, fmt.Errorf("read git remote %q: %w", remote, err)
	}
	raw := strings.TrimSpace(result.Stdout)
	repository, err := ParseGitHubRemote(raw)
	if err != nil {
		return "", GitHubRepository{}, err
	}
	return raw, repository, nil
}

// rejectUnsafeLocalGitConfig keeps repository-controlled Git configuration from
// acquiring transport or host-command authority. This is deliberately checked
// both before policy evaluation and again immediately before execution.
func rejectUnsafeLocalGitConfig(ctx context.Context, runner host.Runner, workspace, remote string) error {
	pattern := `^(remote\.` + regexp.QuoteMeta(remote) + `\.(pushurl|receivepack|proxy)|url\..*\.(insteadOf|pushInsteadOf)|credential\..*|core\.(hooksPath|sshCommand|askPass))$`
	result, err := runner.Run(ctx, "git", "-C", workspace, "config", "--local", "--get-regexp", pattern)
	if err != nil {
		// git config returns 1 when no matching key exists.
		if result.ExitCode == 1 {
			return nil
		}
		return fmt.Errorf("inspect repository-local git security config: %w", err)
	}
	if strings.TrimSpace(result.Stdout) != "" {
		return fmt.Errorf("repository-local git transport or command override is not allowed: %w", core.ErrPolicyDenied)
	}
	return nil
}

func ParseGitHubRemote(raw string) (GitHubRepository, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n\x00") {
		return GitHubRepository{}, core.ErrInvalidArgument
	}
	var path string
	if strings.HasPrefix(raw, "git@github.com:") {
		path = strings.TrimPrefix(raw, "git@github.com:")
	} else {
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "ssh") || !strings.EqualFold(parsed.Hostname(), "github.com") || parsed.RawQuery != "" || parsed.Fragment != "" {
			return GitHubRepository{}, fmt.Errorf("remote must be credential-free github.com HTTPS/SSH URL: %w", core.ErrInvalidArgument)
		}
		if parsed.Scheme == "https" && parsed.User != nil {
			return GitHubRepository{}, fmt.Errorf("GitHub HTTPS remote must not embed credentials: %w", core.ErrInvalidArgument)
		}
		if parsed.Scheme == "ssh" && parsed.User != nil {
			if parsed.User.Username() != "git" {
				return GitHubRepository{}, fmt.Errorf("GitHub SSH user must be git: %w", core.ErrInvalidArgument)
			}
			if _, hasPassword := parsed.User.Password(); hasPassword {
				return GitHubRepository{}, fmt.Errorf("GitHub SSH remote must not embed a password: %w", core.ErrInvalidArgument)
			}
		}
		path = strings.TrimPrefix(parsed.Path, "/")
	}
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || !slugPartPattern.MatchString(parts[0]) || !slugPartPattern.MatchString(parts[1]) {
		return GitHubRepository{}, fmt.Errorf("invalid GitHub repository remote %q: %w", raw, core.ErrInvalidArgument)
	}
	return GitHubRepository{Owner: parts[0], Name: parts[1]}, nil
}

func normalizeBranch(ctx context.Context, runner host.Runner, workspace, branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	branch = strings.TrimPrefix(branch, "refs/heads/")
	if branch == "" || strings.HasPrefix(branch, "-") || strings.ContainsAny(branch, "\r\n\x00") {
		return "", core.ErrInvalidArgument
	}
	if _, err := runner.Run(ctx, "git", "-C", workspace, "check-ref-format", "--branch", branch); err != nil {
		return "", fmt.Errorf("invalid target branch %q: %w", branch, core.ErrInvalidArgument)
	}
	return "refs/heads/" + branch, nil
}

func resolveCommit(ctx context.Context, runner host.Runner, workspace, source string) (string, error) {
	result, err := runner.Run(ctx, "git", "-C", workspace, "rev-parse", "--verify", source+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve source revision %q: %w", source, err)
	}
	sha := strings.ToLower(strings.TrimSpace(result.Stdout))
	if !validObjectID(sha) {
		return "", fmt.Errorf("git returned invalid object id: %w", core.ErrInvalidArgument)
	}
	return sha, nil
}

func ensureCommit(ctx context.Context, runner host.Runner, workspace, sha string) error {
	if _, err := runner.Run(ctx, "git", "-C", workspace, "cat-file", "-e", sha+"^{commit}"); err != nil {
		return fmt.Errorf("approved source object is unavailable: %w", core.ErrCapabilityStale)
	}
	return nil
}

func resolveRemoteRef(ctx context.Context, runner host.Runner, workspace, remoteURL, targetRef string) (string, error) {
	result, err := runner.Run(ctx, "git", "-c", "core.hooksPath=/dev/null", "-C", workspace, "ls-remote", "--refs", remoteURL, targetRef)
	if err != nil {
		return "", fmt.Errorf("inspect remote ref %s: %w", targetRef, err)
	}
	line := strings.TrimSpace(result.Stdout)
	if line == "" {
		return "", nil
	}
	fields := strings.Fields(line)
	if len(fields) != 2 || fields[1] != targetRef || !validObjectID(fields[0]) {
		return "", fmt.Errorf("unexpected ls-remote result: %w", core.ErrInvalidArgument)
	}
	return strings.ToLower(fields[0]), nil
}

func safeRemoteName(remote string) bool {
	return remoteNamePattern.MatchString(remote) && !strings.HasPrefix(remote, "-")
}

func safeRevision(source string) bool {
	return source != "" && !strings.HasPrefix(source, "-") && !strings.ContainsAny(source, "\r\n\x00 \t") && len(source) <= 256
}

func validTargetRef(ref string) bool {
	return strings.HasPrefix(ref, "refs/heads/") && len(ref) > len("refs/heads/") && !strings.ContainsAny(ref, "\r\n\x00 :")
}

func validObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func sanitizedGitDetail(stderr string) string {
	stderr = strings.TrimSpace(stderr)
	if stderr == "" {
		return ""
	}
	if len(stderr) > 500 {
		stderr = stderr[:500]
	}
	return ": " + stderr
}
