package composition

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	agenthostapp "github.com/SLktEx/Hacocoon/internal/agenthost"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	clientapp "github.com/SLktEx/Hacocoon/internal/client"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	gitcapapp "github.com/SLktEx/Hacocoon/internal/gitcap"
	"github.com/SLktEx/Hacocoon/internal/host"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
	seedbuildapp "github.com/SLktEx/Hacocoon/internal/seedbuild"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

type App struct {
	Environments *workspaceapp.Service
	AgentHosts   *agenthostapp.Broker
	Clients      *clientapp.Service
	Capabilities *capabilityapp.Service
	Git          *gitcapapp.Broker
	OCI          *ociplugin.Service
	Seeds        *seedbuildapp.Service
	Runner       *runapp.Service
	Events       *eventsapp.Service
	Bases        *environmentapp.BaseRouter
	Runtime      *incus.Runtime
}

func Local(_ context.Context) (*App, error) {
	runner := host.ExecRunner{}
	root := envOr("HACO_ROOT", "/var/lib/hacocoon")
	stateDir := filepath.Join(root, "state")

	configuredDriver := strings.TrimSpace(os.Getenv("HACO_PLUGIN_OCI"))
	var (
		ociDriver ociplugin.Driver
		seedStore *seedbuildapp.Store
	)
	providerOptions := []incus.BaseProviderOption{}
	if configuredDriver != "" {
		driver, err := ociplugin.ParseDriver(configuredDriver)
		if err != nil {
			return nil, err
		}
		ociDriver = driver
		seedStore = seedbuildapp.NewStore(filepath.Join(stateDir, "seeds.json"))
		providerOptions = append(providerOptions, incus.WithSeedResolver(seedStore))
	}

	incusRuntime := incus.New(runner)
	incusProvider, err := incus.NewSandboxProvider(incusRuntime, providerOptions...)
	if err != nil {
		return nil, err
	}

	router, err := environmentapp.NewRouter(
		envOr("HACO_RUNTIME_PROVIDER", environmentapp.ProviderIncus),
		environmentapp.Register(environmentapp.ProviderIncus, incusProvider),
	)
	if err != nil {
		return nil, err
	}
	runtime := environmentapp.NewBaseRouter(router)

	environmentStatePath := filepath.Join(stateDir, "environments.json")
	store := state.NewEnvironmentJSONStore(environmentStatePath)
	bindingStore := agenthostapp.NewJSONBindingStore(filepath.Join(stateDir, "agent-bindings.json"))
	gitProvider := gitcapapp.NewUnifiedProvider(runner, store)
	auditPath := filepath.Join(root, "audit", "capabilities.jsonl")
	capabilities, err := capabilityapp.New(
		capabilityapp.NewFilePolicyEvaluator(filepath.Join(root, "policy.json")),
		capabilityapp.NewStdioApproval(os.Stdin, os.Stderr),
		capabilityapp.NewJSONLAudit(auditPath),
		capabilityapp.LocalEcho{},
		gitProvider,
	)
	if err != nil {
		return nil, err
	}

	var (
		ociPlugin *ociplugin.Service
		seeds     *seedbuildapp.Service
	)
	if configuredDriver != "" {
		ociPlugin, err = ociplugin.New(
			runtime,
			environmentStatePath,
			ociplugin.NewStore(filepath.Join(stateDir, "oci-usage.json")),
			ociDriver,
			ociplugin.WithHostRunner(runner),
		)
		if err != nil {
			return nil, err
		}
		seeds, err = seedbuildapp.New(incusProvider, ociPlugin, seedStore)
		if err != nil {
			return nil, err
		}
	}

	environments := workspaceapp.New(runtime, store)
	return &App{
		Environments: environments,
		AgentHosts:   agenthostapp.New(environments, store, bindingStore),
		Clients:      clientapp.New(runtime, store),
		Capabilities: capabilities,
		Git:          gitcapapp.NewBroker(runner, store, capabilities),
		OCI:          ociPlugin,
		Seeds:        seeds,
		Runner:       runapp.NewWithRecovery(environments, store, filepath.Join(stateDir, "run-locks")),
		Events:       eventsapp.New(auditPath),
		Bases:        runtime,
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
