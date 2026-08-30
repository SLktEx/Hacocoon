package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	githubcap "github.com/SLktEx/Hacocoon/modules/capability/github"
)

const kubeBrokerSHA = "1111111111111111111111111111111111111111"

type kubeAllowPolicy struct{}

func (kubeAllowPolicy) Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error) {
	return core.PolicyEvaluation{Decision: core.PolicyAllow}, nil
}

type kubeNoApproval struct{}

func (kubeNoApproval) Approve(context.Context, core.ApprovalRequest) (bool, error) {
	return false, errors.New("unexpected approval")
}

type kubeAuditSink struct {
	events []core.CapabilityAuditEvent
}

func (a *kubeAuditSink) Record(_ context.Context, event core.CapabilityAuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

type kubeBrokerGitRunner struct {
	calls       []string
	remoteReads int
	mutateRemote bool
}

func (r *kubeBrokerGitRunner) Run(_ context.Context, name string, args ...string) (host.Result, error) {
	joined := name + " " + strings.Join(args, " ")
	r.calls = append(r.calls, joined)
	if strings.Contains(joined, "config --local --get-regexp") {
		return host.Result{ExitCode: 1}, errors.New("no matching local config")
	}
	switch {
	case strings.Contains(joined, "check-ref-format --branch feature/x"):
		return host.Result{}, nil
	case strings.Contains(joined, "config --get remote.origin.url"):
		r.remoteReads++
		if r.mutateRemote && r.remoteReads > 1 {
			return host.Result{Stdout: "https://github.com/acme/changed.git\n"}, nil
		}
		return host.Result{Stdout: "https://github.com/acme/demo.git\n"}, nil
	case strings.Contains(joined, "rev-parse --verify HEAD^{commit}"):
		return host.Result{Stdout: kubeBrokerSHA + "\n"}, nil
	case strings.Contains(joined, "cat-file -e "+kubeBrokerSHA+"^{commit}"):
		return host.Result{}, nil
	case strings.Contains(joined, "push --porcelain --no-verify https://github.com/acme/demo.git "+kubeBrokerSHA+":refs/heads/feature/x"):
		return host.Result{Stdout: "ok\n"}, nil
	default:
		return host.Result{}, fmt.Errorf("unexpected brokered Git call: %s", joined)
	}
}

func TestKubernetesEnvironmentUsesUnchangedExactSHAPushBroker(t *testing.T) {
	ctx := context.Background()
	workspaceDir := t.TempDir()
	environments, store := newParityEnvironmentService(t, newParityKubeRunner(t))
	created, err := environments.Create(ctx, core.EnvironmentSpec{
		Name:          "demo",
		WorkspacePath: workspaceDir,
		AccessMode:    core.WorkspaceReadWrite,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.RuntimeRef, "haco-runtime-v1:"+ProviderID+":") {
		t.Fatalf("environment did not use Kubernetes provider: %q", created.RuntimeRef)
	}

	t.Setenv("GH_TOKEN", "ambient-gh-token-must-not-cross")
	t.Setenv("GITHUB_TOKEN", "ambient-github-token-must-not-cross")
	t.Setenv("GIT_ASKPASS", "/tmp/untrusted-askpass")

	gitRunner := &kubeBrokerGitRunner{}
	provider := githubcap.NewProvider(gitRunner, store)
	audit := &kubeAuditSink{}
	caps, err := capabilityapp.New(kubeAllowPolicy{}, kubeNoApproval{}, audit, provider)
	if err != nil {
		t.Fatal(err)
	}
	broker := githubcap.NewBroker(gitRunner, store, caps)
	result, err := broker.Push(ctx, githubcap.PushSpec{Environment: "demo", Branch: "feature/x"})
	if err != nil {
		t.Fatalf("broker push: %v\ncalls=%v", err, gitRunner.calls)
	}
	if result.Provider != githubcap.GitHubCapability || result.ExecutionState != core.CapabilitySucceeded {
		t.Fatalf("result = %#v", result)
	}

	var pushed bool
	for _, call := range gitRunner.calls {
		if strings.Contains(call, " push ") {
			pushed = true
			if !strings.Contains(call, kubeBrokerSHA+":refs/heads/feature/x") || strings.Contains(call, " HEAD:refs/heads/feature/x") {
				t.Fatalf("push was not bound to the approved exact SHA: %s", call)
			}
		}
		for _, forbidden := range []string{"ambient-gh-token-must-not-cross", "ambient-github-token-must-not-cross", "/tmp/untrusted-askpass"} {
			if strings.Contains(call, forbidden) {
				t.Fatalf("ambient credential/transport state crossed the trusted Git broker boundary: %s", call)
			}
		}
	}
	if !pushed {
		t.Fatalf("broker did not execute a push: %v", gitRunner.calls)
	}
	if len(audit.events) < 2 {
		t.Fatalf("audit events = %#v", audit.events)
	}
	decision := audit.events[1]
	if decision.Environment != "demo" || decision.Attributes["source_sha"] != kubeBrokerSHA || decision.Attributes["target_ref"] != "refs/heads/feature/x" {
		t.Fatalf("audit decision = %#v", decision)
	}
}

func TestKubernetesEnvironmentPushStillFailsStaleAfterApprovalBoundary(t *testing.T) {
	ctx := context.Background()
	workspaceDir := t.TempDir()
	environments, store := newParityEnvironmentService(t, newParityKubeRunner(t))
	if _, err := environments.Create(ctx, core.EnvironmentSpec{
		Name:          "demo",
		WorkspacePath: workspaceDir,
		AccessMode:    core.WorkspaceReadWrite,
	}); err != nil {
		t.Fatal(err)
	}

	gitRunner := &kubeBrokerGitRunner{mutateRemote: true}
	provider := githubcap.NewProvider(gitRunner, store)
	caps, err := capabilityapp.New(kubeAllowPolicy{}, kubeNoApproval{}, &kubeAuditSink{}, provider)
	if err != nil {
		t.Fatal(err)
	}
	_, err = githubcap.NewBroker(gitRunner, store, caps).Push(ctx, githubcap.PushSpec{Environment: "demo", Branch: "feature/x"})
	if !errors.Is(err, core.ErrCapabilityStale) {
		t.Fatalf("error = %v\ncalls=%v", err, gitRunner.calls)
	}
	for _, call := range gitRunner.calls {
		if strings.Contains(call, " push ") {
			t.Fatalf("push executed after approved remote identity changed: %s", call)
		}
	}
}
