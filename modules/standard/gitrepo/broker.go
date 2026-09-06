package gitrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

type EnvironmentStore interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
}
type CapabilityService interface {
	RequestWithApproval(context.Context, core.CapabilityRequest, func(context.Context, core.ApprovalRequest) (bool, error)) (core.CapabilityResult, error)
}

type Proposal struct {
	ID          string `json:"id"`
	Environment string `json:"environment"`
	Repository  string `json:"repository"`
	Remote      string `json:"remote"`
	Ref         string `json:"ref"`
	OldOID      string `json:"old_oid"`
	NewOID      string `json:"new_oid"`
	Operation   string `json:"operation"`
	Summary     string `json:"summary,omitempty"`
}
type pendingProposal struct {
	proposal Proposal
	decision chan bool
}
type preparedOperation struct {
	request core.CapabilityRequest
	execute func(context.Context) (Response, error)
}
type operationContextKey struct{}
type binding struct {
	Environment  core.Environment `json:"environment"`
	Workspace    Object           `json:"workspace"`
	Repository   Object           `json:"repository"`
	Repositories []Object         `json:"repositories,omitempty"`
}
type boundServer struct {
	binding binding
	server  *http.Server
}

type Broker struct {
	Repositories    *RepositoryService
	Environments    EnvironmentStore
	Capabilities    CapabilityService
	SocketDirectory string
	mu              sync.Mutex
	ctx             context.Context
	servers         map[string]boundServer
	pending         map[string]pendingProposal
	operations      map[string]preparedOperation
}

func NewBroker(repos *RepositoryService, environments EnvironmentStore, sockets string) *Broker {
	return &Broker{Repositories: repos, Environments: environments, SocketDirectory: sockets, servers: map[string]boundServer{}, pending: map[string]pendingProposal{}, operations: map[string]preparedOperation{}}
}
func (*Broker) Capability() string { return Capability }

// Execute accepts only an exact operation prepared inside this broker. The
// general controller Capability API cannot turn supplied paths/URLs into Git.
func (b *Broker) Execute(ctx context.Context, request core.CapabilityRequest) (core.CapabilityResult, error) {
	b.mu.Lock()
	id := request.Attributes["operation_id"]
	operation, ok := b.operations[id]
	if ok && ctx.Value(operationContextKey{}) == id && reflect.DeepEqual(request, operation.request) {
		delete(b.operations, id)
	} else {
		ok = false
	}
	b.mu.Unlock()
	if !ok {
		return core.CapabilityResult{}, core.ErrCapabilityStale
	}
	_, err := operation.execute(ctx)
	return core.CapabilityResult{Provider: Capability, Output: "Git operation completed"}, err
}

func (b *Broker) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.ctx = ctx
	if err := os.MkdirAll(b.SocketDirectory, 0700); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(b.Repositories.Root, "bindings", "*.json"))
	if err != nil {
		return err
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var bound binding
		if len(data) > 16384 || json.Unmarshal(data, &bound) != nil || !ValidID(bound.Environment.Name) {
			return core.ErrIncompatibleState
		}
		if err := b.validateBinding(ctx, bound); err != nil {
			continue
		} // Stale bindings never gain authority.
		if err := b.listenLocked(bound); err != nil {
			return err
		}
	}
	go func() { <-ctx.Done(); b.Close() }()
	return nil
}
func (b *Broker) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, entry := range b.servers {
		entry.server.Close()
		delete(b.servers, id)
	}
}

func (b *Broker) Connect(ctx context.Context, name string) error {
	environment, err := b.Environments.GetEnvironment(ctx, name)
	if err != nil {
		return err
	}
	if !ValidID(name) || !strings.HasPrefix(environment.Workspace.Path, "managed:") {
		return core.ErrInvalidArgument
	}
	workspace, err := b.Repositories.Get("work", strings.TrimPrefix(environment.Workspace.Path, "managed:"))
	if err != nil {
		return err
	}
	bound := binding{Environment: environment, Workspace: workspace}
	for _, member := range workspace.Copies() {
		repo, err := b.Repositories.Get("repo", member.Repository)
		if err != nil {
			return err
		}
		if len(workspace.Members) == 0 {
			bound.Repository = repo
		} else {
			bound.Repositories = append(bound.Repositories, repo)
		}
	}
	if err := b.validateBinding(ctx, bound); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.ctx == nil {
		return core.ErrRuntimeUnavailable
	}
	if old, exists := b.servers[name]; exists && !reflect.DeepEqual(old.binding, bound) {
		// A canonical runtime replacement invalidates the old binding. It may
		// be replaced only after the old Environment identity is stale.
		stale := b.validateBinding(ctx, old.binding)
		if !errors.Is(stale, core.ErrCapabilityStale) && !errors.Is(stale, core.ErrNotFound) {
			return core.ErrIncompatibleState
		}
		old.server.Close()
		delete(b.servers, name)
	}
	if err := writeRecord(filepath.Join(b.Repositories.Root, "bindings", name+".json"), bound); err != nil {
		return err
	}
	if err := b.listenLocked(bound); err != nil {
		return err
	}
	return b.Repositories.Backend.ConnectGit(ctx, environment, workspace, b.socket(name))
}
func (b *Broker) socket(name string) string { return filepath.Join(b.SocketDirectory, name+".sock") }
func (b *Broker) listenLocked(bound binding) error {
	if _, exists := b.servers[bound.Environment.Name]; exists {
		return nil
	}
	listener, err := control.ListenUnix(b.socket(bound.Environment.Name), 0600)
	if err != nil {
		return err
	}
	slot := make(chan struct{}, 1)
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 10 * time.Minute, BaseContext: func(net.Listener) context.Context { return b.ctx }}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case slot <- struct{}{}:
			defer func() { <-slot }()
		default:
			http.Error(w, "busy", http.StatusConflict)
			return
		}
		if r.Method != "POST" || r.URL.Path != "/git" {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 9*time.Minute)
		defer cancel()
		var request Request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxMessage))
		decoder.DisallowUnknownFields()
		var response Response
		err := decoder.Decode(&request)
		if err == nil {
			response, err = b.exchange(ctx, bound, request)
		}
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			logging.FromContext(ctx).ErrorContext(ctx, "Git operation failed", "component", "git", "operation", "git_request", "environment_id", bound.Environment.Name, "error", err)
			w.WriteHeader(http.StatusForbidden)
			response = Response{Error: "operation did not complete cleanly; inspect the remote and trusted Host before retrying"}
		}
		json.NewEncoder(w).Encode(response)
	})
	b.servers[bound.Environment.Name] = boundServer{binding: bound, server: server}
	go server.Serve(listener)
	return nil
}
func (b *Broker) validateBinding(ctx context.Context, bound binding) error {
	current, err := b.Environments.GetEnvironment(ctx, bound.Environment.Name)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(current, bound.Environment) || current.Workspace.ID != core.WorkspaceID("workspace:managed:"+bound.Workspace.Owner) || current.Workspace.Path != "managed:"+bound.Workspace.ID {
		return core.ErrCapabilityStale
	}
	workspace, err := b.Repositories.Get("work", bound.Workspace.ID)
	if err != nil || !reflect.DeepEqual(workspace, bound.Workspace) {
		return core.ErrCapabilityStale
	}
	repos := bound.repositories()
	if len(repos) != len(workspace.Copies()) {
		return core.ErrCapabilityStale
	}
	for i, member := range workspace.Copies() {
		repo, err := b.Repositories.Get("repo", member.Repository)
		if err != nil || !reflect.DeepEqual(repo, repos[i]) {
			return core.ErrCapabilityStale
		}
	}
	return nil
}

func (bound binding) repositories() []Object {
	if len(bound.Workspace.Members) != 0 {
		return bound.Repositories
	}
	return []Object{bound.Repository}
}

func (b *Broker) exchange(ctx context.Context, bound binding, req Request) (Response, error) {
	if err := b.validateBinding(ctx, bound); err != nil {
		return Response{}, err
	}
	var repo Object
	for _, candidate := range bound.repositories() {
		if candidate.ID == req.Repository {
			repo = candidate
			break
		}
	}
	ref := "refs/heads/" + repo.Branch
	if repo.ID == "" || req.Repository != repo.ID || len(req.Pack) > MaxPack {
		return Response{}, core.ErrPolicyDenied
	}
	agent := AgentRequest{Operation: req.Operation, Repository: repo.ID, Remote: repo.Remote, Branch: repo.Branch, OldOID: req.OldOID, NewOID: req.NewOID, Pack: req.Pack}
	switch req.Operation {
	case "list":
		if req.Ref != "" || req.OldOID != "" || req.NewOID != "" || len(req.Pack) != 0 {
			return Response{}, core.ErrInvalidArgument
		}
	case "fetch":
		if req.Ref != ref || !ValidOID(req.NewOID) || req.OldOID != "" || len(req.Pack) != 0 {
			return Response{}, core.ErrInvalidArgument
		}
	case "push":
		if req.Ref != ref || !ValidOID(req.NewOID) || !ValidOID(req.OldOID) || len(req.Pack) == 0 {
			return Response{}, core.ErrInvalidArgument
		}
	default:
		return Response{}, core.ErrUnsupported
	}
	proposal := Proposal{Environment: bound.Environment.Name, Repository: repo.ID, Remote: repo.Remote, Ref: ref, OldOID: req.OldOID, NewOID: req.NewOID, Operation: "fetch"}
	if req.Operation != "push" {
		return b.perform(ctx, bound, proposal, func(ctx context.Context) (Response, error) { return b.Repositories.Backend.RunGit(ctx, agent) })
	}
	// Fetch authorization covers the remote observation used to prepare a push.
	// It does not authorize the later external write.
	agent.Operation = "prepare"
	prepared, err := b.perform(ctx, bound, proposal, func(ctx context.Context) (Response, error) { return b.Repositories.Backend.RunGit(ctx, agent) })
	if err != nil {
		return Response{}, err
	}
	proposal.Operation = "push"
	proposal.Summary = prepared.Summary
	agent.Operation = "push"
	agent.Pack = nil
	return b.perform(ctx, bound, proposal, func(ctx context.Context) (Response, error) { return b.Repositories.Backend.RunGit(ctx, agent) })
}

func (b *Broker) perform(ctx context.Context, bound binding, proposal Proposal, execute func(context.Context) (Response, error)) (Response, error) {
	if b.Capabilities == nil {
		return Response{}, core.ErrPolicyDenied
	}
	proposal.ID = randomID()
	request := core.CapabilityRequest{Capability: Capability, Action: proposal.Operation, Environment: proposal.Environment, Resource: proposal.Remote, Attributes: map[string]string{"repository": proposal.Repository, "remote": proposal.Remote, "target_ref": proposal.Ref, "old_oid": proposal.OldOID, "new_oid": proposal.NewOID, "operation_id": proposal.ID}}
	var response Response
	operation := preparedOperation{request: request, execute: func(ctx context.Context) (Response, error) {
		if err := ctx.Err(); err != nil {
			return Response{}, err
		}
		if err := b.validateBinding(ctx, bound); err != nil {
			return Response{}, err
		}
		var err error
		response, err = execute(ctx)
		return response, err
	}}
	b.mu.Lock()
	b.operations[proposal.ID] = operation
	b.mu.Unlock()
	defer func() { b.mu.Lock(); delete(b.operations, proposal.ID); delete(b.pending, proposal.ID); b.mu.Unlock() }()
	ctx = context.WithValue(ctx, operationContextKey{}, proposal.ID)
	_, err := b.Capabilities.RequestWithApproval(ctx, request, func(ctx context.Context, _ core.ApprovalRequest) (bool, error) {
		decision := make(chan bool, 1)
		b.mu.Lock()
		b.pending[proposal.ID] = pendingProposal{proposal: proposal, decision: decision}
		b.mu.Unlock()
		select {
		case approved := <-decision:
			return approved, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	})
	return response, err
}
func (b *Broker) Pending() []Proposal {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Proposal, 0, len(b.pending))
	for _, pending := range b.pending {
		result = append(result, pending.proposal)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}
func (b *Broker) Decide(id string, approved bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	pending, ok := b.pending[id]
	if !ok {
		return fmt.Errorf("approval is no longer pending: %w", core.ErrNotFound)
	}
	delete(b.pending, id)
	pending.decision <- approved
	return nil
}
