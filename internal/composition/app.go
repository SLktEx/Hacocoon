package composition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agenthostapp "github.com/SLktEx/Hacocoon/internal/agenthost"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	clientapp "github.com/SLktEx/Hacocoon/internal/client"
	"github.com/SLktEx/Hacocoon/internal/core"
	egressapp "github.com/SLktEx/Hacocoon/internal/egress"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	gitcapapp "github.com/SLktEx/Hacocoon/internal/gitcap"
	"github.com/SLktEx/Hacocoon/internal/host"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
	seedbuildapp "github.com/SLktEx/Hacocoon/internal/seedbuild"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storagepriv"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
	"github.com/SLktEx/Hacocoon/modules/standard/egressproxy"
	storagebtrfs "github.com/SLktEx/Hacocoon/modules/storage/btrfs"
)

const defaultLocalStorageID = "local-default"

const defaultLocalStorageBytes int64 = 128 << 30

type App struct {
	Environments *workspaceapp.Service
	AgentHosts    *agenthostapp.Broker
	Clients       *clientapp.Service
	Capabilities  *capabilityapp.Service
	Git           *gitcapapp.Broker
	OCI           *ociplugin.Service
	Seeds         *seedbuildapp.Service
	Runner        *runapp.Service
	Events        *eventsapp.Service
	Bases         *environmentapp.BaseRouter
	Runtime       *incus.Runtime
	EgressProxy   *egressproxy.Proxy
}

func Local(ctx context.Context) (*App, error) {
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

	var storageRunner host.Runner = runner
	switch mode := strings.TrimSpace(os.Getenv("HACO_STORAGE_PRIVILEGE_MODE")); mode {
	case "", "sudo":
		privilegedRunner, err := storagepriv.NewSudoRunner(root, runner)
		if err != nil {
			return nil, err
		}
		storageRunner = privilegedRunner
	case "direct":
		// Test/development escape hatch. This never grants authority: every
		// operation runs with the caller's existing credentials and therefore
		// fails on a normal Host when privilege is actually required.
		storageRunner = runner
	default:
		return nil, fmt.Errorf("unknown HACO_STORAGE_PRIVILEGE_MODE %q", mode)
	}

	managedStorage, err := storagebtrfs.NewLocal(ctx, root, storageRunner, strings.TrimSpace(os.Getenv("HACO_BLOCK_BACKEND")))
	if err != nil {
		return nil, err
	}

	var runtimeRunner host.Runner = runner
	if ociDriver == ociplugin.DriverNerdctl {
		runtimeRunner = incus.WrapSeedHarvestRunner(runner)
	}
	incusRuntime := incus.New(runtimeRunner)
	if err := incusRuntime.ConfigureStorageProvider(func(storageCtx context.Context) (map[string]string, error) {
		handle, err := managedStorage.Ensure(storageCtx, core.StorageSpec{
			ID:        defaultLocalStorageID,
			SizeBytes: defaultLocalStorageBytes,
		})
		if err != nil {
			return nil, err
		}
		return handle.Attachment, nil
	}); err != nil {
		return nil, err
	}
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
		egressapp.Provider{},
		gitProvider,
	)
	if err != nil {
		return nil, err
	}
	egressBroker := egressapp.NewBroker(capabilities)
	egressSources, err := egressapp.NewPersistedSourceResolver(incusRuntime, store)
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
		AgentHosts:    agenthostapp.New(environments, store, bindingStore),
		Clients:       clientapp.New(runtime, store),
		Capabilities:  capabilities,
		Git:           gitcapapp.NewBroker(runner, store, capabilities),
		OCI:           ociPlugin,
		Seeds:         seeds,
		Runner:        runapp.NewWithRecovery(environments, store, filepath.Join(stateDir, "run-locks")),
		Events:        eventsapp.New(auditPath),
		Bases:         runtime,
		Runtime:       incusRuntime,
		EgressProxy:   egressproxy.New(egressBroker, egressSources),
	}, nil
}

func envOr(name, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return fallback
}
