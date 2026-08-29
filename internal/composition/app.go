package composition

import (
	"context"
	"os"
	"path/filepath"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	clientapp "github.com/SLktEx/Hacocoon/internal/client"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	gitcapapp "github.com/SLktEx/Hacocoon/internal/gitcap"
	"github.com/SLktEx/Hacocoon/internal/host"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
	"github.com/SLktEx/Hacocoon/internal/state"
	workspaceapp "github.com/SLktEx/Hacocoon/internal/workspace"
	"github.com/SLktEx/Hacocoon/modules/runtime/incus"
)

type App struct {
	Environments *workspaceapp.Service
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
	runtime := incus.New(runner)
	store := state.NewEnvironmentJSONStore(filepath.Join(root, "state", "environments.json"))
	gitProvider := gitcapapp.NewProvider(runner, store)
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
	environments := workspaceapp.New(runtime, store)
	return &App{
		Environments: environments,
		Clients:      clientapp.New(runtime, store),
		Capabilities: capabilities,
		Git:          gitcapapp.NewBroker(runner, store, capabilities),
		Runner:       runapp.New(environments),
		Events:       eventsapp.New(auditPath),
		Runtime:      runtime,
	}, nil
}

func envOr(name, fallback string) string {
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	return fallback
}
