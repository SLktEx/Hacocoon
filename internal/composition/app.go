package composition

import (
	"context"
	ocitooling "github.com/SLktEx/Hacocoon/internal/adapters/oci"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"

	awsplugin "github.com/SLktEx/Hacocoon/internal/adapters/aws"
	"github.com/SLktEx/Hacocoon/internal/adapters/incus"
	"github.com/SLktEx/Hacocoon/internal/adapters/network/dns"
	"github.com/SLktEx/Hacocoon/internal/adapters/network/proxy"
	packerplugin "github.com/SLktEx/Hacocoon/internal/adapters/packer"
	agenthostapp "github.com/SLktEx/Hacocoon/internal/agenthost"
	"github.com/SLktEx/Hacocoon/internal/base/build"
	"github.com/SLktEx/Hacocoon/internal/base/manage"
	clientapp "github.com/SLktEx/Hacocoon/internal/client"
	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/env"
	"github.com/SLktEx/Hacocoon/internal/env/copy"
	runapp "github.com/SLktEx/Hacocoon/internal/env/run"
	"github.com/SLktEx/Hacocoon/internal/env/setup"
	"github.com/SLktEx/Hacocoon/internal/env/transfer"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	"github.com/SLktEx/Hacocoon/internal/git"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/host/recipes"
	"github.com/SLktEx/Hacocoon/internal/network"
	"github.com/SLktEx/Hacocoon/internal/network/dns"
	egressapp "github.com/SLktEx/Hacocoon/internal/network/egress"
	capabilityapp "github.com/SLktEx/Hacocoon/internal/policy"
	"github.com/SLktEx/Hacocoon/internal/policy/approvals"
	"github.com/SLktEx/Hacocoon/internal/policy/review"
	"github.com/SLktEx/Hacocoon/internal/snapshot/restore"
	"github.com/SLktEx/Hacocoon/internal/state"
	"github.com/SLktEx/Hacocoon/internal/storage/cache"
	ociplugin "github.com/SLktEx/Hacocoon/internal/storage/oci"
	"github.com/SLktEx/Hacocoon/internal/storage/resource"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/internal/workspace/workflow"
)

const defaultLocalStorageID = "local-default"

const defaultLocalStorageSize = "128GiB"

const defaultLocalStorageMountOptions = "compress=zstd:3,noatime,nodiscard"

type App struct {
	Cache               *cache.Workflow
	hostSetupActive     sync.Mutex
	hostSetupDone       chan struct{}
	Workflow            *workflow.Service
	Networks            *networkrelay.Service
	transferCatalog     *state.EnvironmentJSONStore
	EnvironmentCopy     *environmentcopy.Service
	BaseBuild           *basebuild.Service
	BaseManage          *basemanage.Service
	SnapshotRestore     *snapshotrestore.Service
	AWS                 *awsplugin.Broker
	Reviews             *review.Service
	Configuration       *capabilityapp.PolicyConfiguration
	HostCustomization   *recipes.HostService
	ProjectSetup        *projectsetup.Service
	Environments        *workspaceapp.Service
	AgentHosts          *agenthostapp.Broker
	Clients             *clientapp.Service
	Capabilities        *capabilityapp.Service
	Runner              *runapp.Service
	Events              *eventsapp.Service
	Bases               *environmentapp.BaseRouter
	Runtime             *incus.Runtime
	EgressProxy         *egressproxy.Proxy
	Repositories        *gitrepo.RepositoryService
	GitBroker           *gitrepo.Broker
	PersistentResources *persistentresource.Service
	OCIImages           *ociplugin.ManagedImages
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

	var runtimeRunner host.Runner = runner
	// Environment bridge ownership is enforced at the production Incus command
	// boundary so future call sites cannot silently adopt/delete a same-named
	// unmanaged bridge even if they bypass a higher-level network helper.
	runtimeRunner = incus.WrapEnvironmentNetworkOwnershipRunner(runtimeRunner)
	incusRuntime := incus.New(runtimeRunner)
	maintenanceTools := &ocitooling.MaintenanceTooling{Directory: filepath.Join(root, "oci-maintenance-tools")}
	incusRuntime.ConfigureMaintenanceTooling(maintenanceTools.Prepare)
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

	incusProvider, err := incus.NewSandboxProvider(incusRuntime)
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
	repositoryBackend := &incus.RepositoryBackend{Runtime: incusRuntime, ImportRoot: filepath.Join(root, "transfers"), ImportLimit: environmenttransfer.DefaultPayloadLimit, ProductBinary: filepath.Join(filepath.Dir(executable), "haco")}
	repositories := gitrepo.NewRepositoryService(filepath.Join(stateDir, "repositories"), repositoryBackend)
	repositories.SnapshotCatalog = store
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
	auditPath := filepath.Join(root, "audit", "capabilities.jsonl")
	policy := capabilityapp.NewFilePolicyEvaluator(filepath.Join(root, "policy.json"))
	audit := capabilityapp.NewJSONLAudit(auditPath)
	capabilities, err := capabilityapp.New(
		policy,
		approval,
		audit,
		capabilityapp.LocalEcho{},
		egressapp.Provider{},
		dnsproxy.Provider{Environments: store, Backend: router},
		networkrelay.Provider{},
		&awsplugin.Provider{Host: incusRuntime.RunTrustedHostPython, Stream: incusRuntime.RunTrustedHostPythonStream},
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

	environments := workspaceapp.NewWithProvider(runtime, store, repositoryWorkspaceProvider{repositories: repositories})
	resources := &persistentresource.Service{Store: store, Backend: &incus.PersistentResourceBackend{Runtime: incusRuntime, ImportRoot: filepath.Join(root, "transfers"), ImportLimit: environmenttransfer.DefaultPayloadLimit}}
	workspaceStores := ociplugin.WorkspaceStores{Resources: resources}
	incusRuntime.ConfigureHostCopyRecovery(workspaceStores.RecoverHostCopies)
	incusRuntime.ConfigureHostStorage(func(ctx context.Context) error {
		backend := &incus.PersistentResourceBackend{Runtime: incusRuntime}
		if err := workspaceStores.EnsureHost(ctx, backend); err != nil {
			return err
		}
		source, err := store.GetPersistentResource(ctx, ociplugin.HostStoreID)
		if err != nil {
			return err
		}
		if err := backend.EnableHostOCI(ctx, source); err != nil {
			return err
		}
		return backend.ProvisionHostTools(ctx, source)
	})
	environments.ConfigureDefaultResource(workspaceStores.Resolve)
	cacheSettings := cache.Settings{Path: filepath.Join(stateDir, "cache.json")}
	environments.ConfigureEnvironmentResources(resources, func(ctx context.Context, request core.EnvironmentResourceRequest) ([]core.EnvironmentResourceSelection, error) {
		selector, err := cacheSettings.Select(ctx, store, repositories)
		if err != nil {
			return nil, err
		}
		return selector.Select(ctx, request)
	})
	runs := runapp.NewWithRecovery(environments, store, filepath.Join(stateDir, "run-locks"))
	runs.ConfigureTemporaryWorkspace(workspaceStores.CleanupTemporary)
	restorer := &snapshotrestore.Service{Catalog: store, Environments: environments, Workspaces: repositories, Stores: resources}
	awsBroker := &awsplugin.Broker{Host: incusRuntime.RunTrustedHostPython, Capabilities: capabilities, Environments: store}
	configuration := &capabilityapp.PolicyConfiguration{Evaluator: policy, Audit: audit}
	resolver := nameresolution.New(capabilities)
	networkAuthority := &networkrelay.ConfiguredAuthority{Catalog: store, Configuration: configuration, Policy: policy, Audit: audit}
	networks := &networkrelay.Service{
		Capabilities: capabilities, Authority: networkAuthority,
		Targets: &networkrelay.ConfiguredTargets{Configuration: configuration, Authority: networkAuthority, DNS: resolver, ExternalAllowed: incusRuntime.ExternalNetworkAddress, HostAllowed: incusRuntime.HostNetworkAddress},
		DialTarget: func(ctx context.Context, target networkrelay.Target, address netip.Addr) (net.Conn, error) {
			if target.Kind == "environment" {
				env, err := store.GetEnvironment(ctx, target.Name)
				if err != nil {
					return nil, err
				}
				instance, err := store.EnvironmentInstance(ctx, env)
				if err != nil || instance != target.Owner {
					return nil, core.ErrCapabilityStale
				}
				return runtime.DialEnvironmentNetwork(ctx, env.RuntimeRef, target.Owner, target.Protocol, target.Port)
			}
			return incusRuntime.DialDevelopmentNetwork(ctx, address, target.Protocol, target.Port, target.Kind == "host")
		},
	}
	operations := http.NewServeMux()
	operations.Handle(networkrelay.Path, &networkrelay.Handler{Service: networks, Sources: egressSources})
	operations.Handle("/", awsplugin.NewGuestHandler(awsBroker, egressSources))

	return &App{
		Cache:               &cache.Workflow{Settings: cacheSettings, Catalog: store, Collector: environments, Cleaner: resources, Recoverer: resources},
		Workflow:            &workflow.Service{Repositories: repositories, Environments: environments, Stores: resources},
		Networks:            networks,
		transferCatalog:     store,
		SnapshotRestore:     restorer,
		BaseBuild:           &basebuild.Service{Environments: environments, Packer: packerplugin.Runner{}},
		BaseManage:          &basemanage.Service{Backend: incusProvider.BaseProvider, Catalog: store},
		EnvironmentCopy:     &environmentcopy.Service{Catalog: store, Snapshots: environments, Restorer: restorer},
		AWS:                 awsBroker,
		ProjectSetup:        &projectsetup.Service{Root: filepath.Join(root, "project-setup"), Environments: environments},
		HostCustomization:   &recipes.HostService{Root: filepath.Join(root, "host-customization"), Identity: incusRuntime.TrustedHostIdentity, Execute: incusRuntime.RunTrustedHostCustomization},
		PersistentResources: resources,
		OCIImages: &ociplugin.ManagedImages{Catalog: store, Environments: environments, Host: &incus.PersistentResourceBackend{Runtime: incusRuntime}, Maintain: func(ctx context.Context, resource core.PersistentResourceRef, operation func(context.Context, core.Environment) error) error {
			_, err := runs.MaintainResource(ctx, resource, operation)
			return err
		}},
		Environments:  environments,
		AgentHosts:    agenthostapp.New(environments, store, bindingStore),
		Clients:       clientapp.NewWithLifecycle(runtime, store, environments),
		Capabilities:  capabilities,
		Configuration: configuration,
		Runner:        runs,
		Events:        eventsapp.New(auditPath),
		Bases:         runtime,
		Runtime:       incusRuntime,
		EgressProxy:   egressproxy.NewWithOperations(egressBroker, egressSources, resolver, operations),
		Repositories:  repositories,
		GitBroker:     gitBroker,
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
