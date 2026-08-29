package composition

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	agenthostapp "github.com/SLktEx/Hacocoon/internal/agenthost"
	awscapapp "github.com/SLktEx/Hacocoon/internal/awscap"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	clientapp "github.com/SLktEx/Hacocoon/internal/client"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	gitcapapp "github.com/SLktEx/Hacocoon/internal/gitcap"
	"github.com/SLktEx/Hacocoon/internal/host"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	ec2runtime "github.com/SLktEx/Hacocoon/modules/runtime/ec2"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

type App struct {
	Environments *workspaceapp.Service
	AgentHosts   *agenthostapp.Broker
	Clients      *clientapp.Service
	Capabilities *capabilityapp.Service
	Git          *gitcapapp.Broker
	Runner       *runapp.Service
	Events       *eventsapp.Service
	Runtime      *incus.Runtime
}

func Local(_ context.Context) (*App, error) {
	runner := host.ExecRunner{}
	root := envOr("HACO_ROOT", "/var/lib/hacocoon")
	incusRuntime := incus.New(runner)

	var ec2Provider environmentapp.Provider = environmentapp.DisabledProvider{
		ID:     environmentapp.ProviderEC2,
		Reason: "experimental EC2 is disabled; set HACO_EXPERIMENTAL_EC2=1 to opt in",
	}
	if strings.TrimSpace(os.Getenv("HACO_EXPERIMENTAL_EC2")) == "1" {
		ec2Provider = ec2runtime.New(runner, ec2runtime.ConfigFromEnv())
	}
	runtime, err := environmentapp.NewRouter(
		envOr("HACO_RUNTIME_PROVIDER", environmentapp.ProviderIncus),
		environmentapp.Register(environmentapp.ProviderIncus, incusRuntime),
		environmentapp.Register(environmentapp.ProviderEC2, ec2Provider),
	)
	if err != nil {
		return nil, err
	}

	store := state.NewEnvironmentJSONStore(filepath.Join(root, "state", "environments.json"))
	bindingStore := agenthostapp.NewJSONBindingStore(filepath.Join(root, "state", "agent-bindings.json"))
	gitProvider := gitcapapp.NewProvider(runner, store)
	auditPath := filepath.Join(root, "audit", "capabilities.jsonl")
	capabilities, err := capabilityapp.New(
		capabilityapp.NewFilePolicyEvaluator(filepath.Join(root, "policy.json")),
		capabilityapp.NewStdioApproval(os.Stdin, os.Stderr),
		capabilityapp.NewJSONLAudit(auditPath),
		capabilityapp.LocalEcho{},
		gitProvider,
		awscapapp.NewProvider(runner),
	)
	if err != nil {
		return nil, err
	}
	environments := workspaceapp.New(runtime, store)
	return &App{
		Environments: environments,
		AgentHosts:   agenthostapp.New(environments, store, bindingStore),
		Clients:      clientapp.New(runtime, store),
		Capabilities: capabilities,
		Git:          gitcapapp.NewBroker(runner, store, capabilities),
		Runner:       runapp.New(environments),
		Events:       eventsapp.New(auditPath),
		Runtime:      incusRuntime,
	}, nil
}

func envOr(name, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return fallback
}
