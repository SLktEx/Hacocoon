package composition

import (
	"context"
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
	"github.com/SLktEx/Hacocoon/internal/nameresolution"
	"github.com/SLktEx/Hacocoon/internal/persistentresource"
	"github.com/SLktEx/Hacocoon/internal/recipes"
	"github.com/SLktEx/Hacocoon/internal/review"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
	seedbuildapp "github.com/SLktEx/Hacocoon/internal/seedbuild"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	ociplugin "github.com/SLktEx/Hacocoon/modules/plugin/oci"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
	"github.com/SLktEx/Hacocoon/modules/standard/approvals"
	"github.com/SLktEx/Hacocoon/modules/standard/dnsproxy"
	"github.com/SLktEx/Hacocoon/modules/standard/egressproxy"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"github.com/SLktEx/Hacocoon/modules/standard/projectsetup"
)

const defaultLocalStorageID = "local-default"

const defaultLocalStorageSize = "128GiB"

const defaultLocalStorageMountOptions = "compress=zstd:3,noatime,nodiscard"

type App struct {
	Reviews             *review.Service
	Configuration       *capabilityapp.PolicyConfiguration
	HostCustomization   *recipes.Service
	ProjectSetup        *projectsetup.Service
	Environments        *workspaceapp.Service
	AgentHosts          *agenthostapp.Broker
	Clients             *clientapp.Service
	Capabilities        *capabilityapp.Service
	Git                 *gitcapapp.Broker
	OCI                 *ociplugin.Service
	Seeds               *seedbuildapp.Service
	Runner              *runapp.Service
	Events              *eventsapp.Service
	Bases               *environmentapp.BaseRouter
	Runtime             *incus.Runtime
	EgressProxy         *egressproxy.Proxy
	Repositories        *gitrepo.RepositoryService
	GitBroker           *gitrepo.Broker
	PersistentResources *persistentresource.Service
}

func Local(ctx context.Context) (*App, error) {
	return local(ctx, capabilityapp.NewStdioApproval(os.Stdin, os.Stderr))
}

// Controller never consumes ambient stdin. Background requests requiring human
// approval wait in a bounded Standard queue, reviewed through the private API.
// Interactive control sessions retain their own scoped approval callback.
func Controller(ctx context.Context) (*App, error) {
	queue := approvals.New()
	app, err := local(ctx, queue)
	if err != nil {
		return nil, err
	}
	app.Reviews = review.New(queue, app.GitBroker)
	return app, nil
}

func local(ctx context.Context, approval capabilityapp.ApprovalProvider) (*App, error) {
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

	var runtimeRunner host.Runner = runner
	if ociDriver == ociplugin.DriverNerdctl {
		runtimeRunner = incus.WrapSeedHarvestRunner(runner)
	}
	// Environment bridge ownership is enforced at the production Incus command
	// boundary so future call sites cannot silently adopt/delete a same-named
	// unmanaged bridge even if they bypass a higher-level network helper.
	runtimeRunner = incus.WrapEnvironmentNetworkOwnershipRunner(runtimeRunner)
	incusRuntime := incus.New(runtimeRunner)
	if kernel, err := os.ReadFile("/proc/sys/kernel/osrelease"); err == nil && strings.Contains(strings.ToLower(string(kernel)), "microsoft") {
		incusRuntime.ConfigureWSLInterop()
	}

	// Incus is the single lifecycle owner for the default local Btrfs pool,
	// including its backing image, loop device, filesystem, mount, and resize.
	if err := incusRuntime.ConfigureStorageProvider(func(storageCtx context.Context) (map[string]string, error) {
		return ensureDefaultIncusStoragePool(storageCtx, runtimeRunner)
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
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	repositoryBackend := &incus.RepositoryBackend{Runtime: incusRuntime, ProductBinary: filepath.Join(filepath.Dir(executable), "haco")}
	repositories := gitrepo.NewRepositoryService(filepath.Join(stateDir, "repositories"), repositoryBackend)
	incusRuntime.ConfigureManagedWorkspaces(func(ctx context.Context, source string) ([]incus.WorkspaceAttachment, error) {
		if !strings.HasPrefix(source, "managed:") {
			return nil, core.ErrInvalidArgument
		}
		object, err := repositories.Get("work", strings.TrimPrefix(source, "managed:"))
		if err != nil {
			return nil, err
		}
		return repositoryBackend.WorkspaceAttachments(ctx, object)
	})
	gitBroker := gitrepo.NewBroker(repositories, store, filepath.Join(root, "run", "git"))
	bindingStore := agenthostapp.NewJSONBindingStore(filepath.Join(stateDir, "agent-bindings.json"))
	gitProvider := gitcapapp.NewUnifiedProvider(runner, store)
	auditPath := filepath.Join(root, "audit", "capabilities.jsonl")
	policy := capabilityapp.NewFilePolicyEvaluator(filepath.Join(root, "policy.json"))
	audit := capabilityapp.NewJSONLAudit(auditPath)
	capabilities, err := capabilityapp.New(
		policy,
		approval,
		audit,
		capabilityapp.LocalEcho{},
		egressapp.Provider{},
		dnsproxy.Provider{},
		gitProvider,
		gitBroker,
	)
	if err != nil {
		return nil, err
	}
	capabilities.ConfigureEnvironmentIdentity(store)
	gitBroker.Capabilities = capabilities
	egressBroker := egressapp.NewBroker(capabilities)
	egressSources, err := egressapp.NewPersistedSourceResolver(environmentapp.ProviderIncus, incusRuntime, store)
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

	environments := workspaceapp.NewWithProvider(runtime, store, repositoryWorkspaceProvider{repositories: repositories})
	resources := &persistentresource.Service{Store: store, Backend: &incus.PersistentResourceBackend{Runtime: incusRuntime}}
	workspaceStores := ociplugin.WorkspaceStores{Resources: resources}
	incusRuntime.ConfigureHostStorage(func(ctx context.Context) error {
		return workspaceStores.EnsureHost(ctx, &incus.PersistentResourceBackend{Runtime: incusRuntime})
	})
	environments.ConfigureDefaultResource(workspaceStores.Resolve)
	runs := runapp.NewWithRecovery(environments, store, filepath.Join(stateDir, "run-locks"))
	runs.ConfigureTemporaryWorkspace(workspaceStores.CleanupTemporary)
	return &App{
		ProjectSetup:        &projectsetup.Service{Root: filepath.Join(root, "project-setup"), Environments: environments},
		HostCustomization:   &recipes.Service{Root: filepath.Join(root, "host-customization"), Execute: incusRuntime.RunTrustedHostCustomization},
		PersistentResources: resources,
		Environments:        environments,
		AgentHosts:          agenthostapp.New(environments, store, bindingStore),
		Clients:             clientapp.New(runtime, store),
		Capabilities:        capabilities,
		Configuration:       &capabilityapp.PolicyConfiguration{Evaluator: policy, Audit: audit},
		Git:                 gitcapapp.NewBroker(runner, store, capabilities),
		OCI:                 ociPlugin,
		Seeds:               seeds,
		Runner:              runs,
		Events:              eventsapp.New(auditPath),
		Bases:               runtime,
		Runtime:             incusRuntime,
		EgressProxy:         egressproxy.NewWithNameResolution(egressBroker, egressSources, nameresolution.New(capabilities)),
		Repositories:        repositories,
		GitBroker:           gitBroker,
	}, nil
}

func defaultIncusStorageAttachment() map[string]string {
	return map[string]string{
		"incus_pool":          "haco-" + defaultLocalStorageID,
		"driver":              "btrfs",
		"size":                defaultLocalStorageSize,
		"btrfs.mount_options": defaultLocalStorageMountOptions,
	}
}

func envOr(name, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return fallback
}
