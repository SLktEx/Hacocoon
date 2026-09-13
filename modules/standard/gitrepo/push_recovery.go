package gitrepo

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"time"

	capabilityapp "github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	pushStarted   = "git-push-started"
	pushConfirmed = "git-push-confirmed"
)

type AuditHistory interface {
	StreamAudit(context.Context, int64, func(core.CapabilityAuditEvent, int64) error) (int64, error)
}

// PushStatus describes durable facts, not permission to retry. A matching
// remote OID never substitutes for the original push's porcelain receipt.
type PushStatus struct {
	Found       bool      `json:"found"`
	RequestID   string    `json:"request_id,omitempty"`
	Environment string    `json:"environment,omitempty"`
	Repository  string    `json:"repository,omitempty"`
	Ref         string    `json:"ref,omitempty"`
	OldOID      string    `json:"old_oid,omitempty"`
	NewOID      string    `json:"new_oid,omitempty"`
	State       string    `json:"state,omitempty"`
	Active      bool      `json:"active"`
	Dispatched  bool      `json:"dispatch_recorded"`
	Completed   *bool     `json:"capability_completed_successfully,omitempty"`
	ObservedOID string    `json:"observed_oid,omitempty"`
	Observation string    `json:"observation,omitempty"`
	ObservedAt  time.Time `json:"observed_at,omitzero"`
}

func (b *Broker) beginPush(ctx context.Context, bound binding, request core.CapabilityRequest) (core.CapabilityAuditEvent, error) {
	event := core.CapabilityAuditEvent{Time: time.Now().UTC(), Type: pushStarted,
		RequestID: capabilityapp.ExecutionRequestID(ctx), Capability: request.Capability,
		Action: request.Action, Resource: request.Resource, Environment: request.Environment,
		EnvironmentInstance: request.EnvironmentInstance, Attributes: maps.Clone(request.Attributes)}
	for _, repo := range bound.repositories() {
		if repo.ID == request.Attributes["repository"] {
			event.Attributes["repository_owner"] = repo.Owner
		}
	}
	if b.PushAudit == nil || event.RequestID == "" || !core.ValidEnvironmentInstanceID(event.EnvironmentInstance) || !validPushAudit(event) || !ValidID(event.Attributes["repository_owner"]) {
		return event, core.ErrRecoveryRequired
	}
	if err := b.PushAudit.Record(ctx, event); err != nil {
		return event, errors.Join(core.ErrAuditIncomplete, err)
	}
	return event, nil
}

func validPushAudit(event core.CapabilityAuditEvent) bool {
	a := event.Attributes
	return event.RequestID != "" && event.Capability == Capability && event.Action == "push" &&
		ValidID(event.Environment) && ValidID(a["repository"]) && ValidID(a["operation_id"]) &&
		ValidateRemote(event.Resource) == nil && a["remote"] == event.Resource &&
		validHeadRef(a["target_ref"]) && ValidOID(a["old_oid"]) && ValidOID(a["new_oid"]) && a["new_oid"] != ZeroOID
}

func (b *Broker) PushStatus(ctx context.Context, environment, requestID string) (PushStatus, error) {
	status, _, err := b.pushStatus(ctx, environment, requestID)
	return status, err
}

func (b *Broker) pushStatus(ctx context.Context, environment, requestID string) (PushStatus, core.CapabilityAuditEvent, error) {
	var status PushStatus
	var requested, started core.CapabilityAuditEvent
	if !ValidID(environment) || len(requestID) > 256 {
		return status, started, core.ErrInvalidArgument
	}
	if b.AuditHistory == nil {
		return status, started, core.ErrRuntimeUnavailable
	}
	_, err := b.AuditHistory.StreamAudit(ctx, 0, func(event core.CapabilityAuditEvent, _ int64) error {
		if event.Capability != Capability || event.Action != "push" || event.Environment != environment || (requestID != "" && event.RequestID != requestID) {
			return nil
		}
		if !validPushAudit(event) {
			return core.ErrRecoveryRequired
		}
		if event.Type == "requested" {
			if status.Found && event.RequestID == status.RequestID {
				return core.ErrRecoveryRequired
			}
			requested, started = event, core.CapabilityAuditEvent{}
			status = PushStatus{Found: true, RequestID: event.RequestID, Environment: environment,
				Repository: event.Attributes["repository"], Ref: event.Attributes["target_ref"],
				OldOID: event.Attributes["old_oid"], NewOID: event.Attributes["new_oid"], State: "unconfirmed"}
			return nil
		}
		if !status.Found || event.RequestID != status.RequestID {
			return nil
		}
		if event.EnvironmentInstance != requested.EnvironmentInstance || event.Resource != requested.Resource {
			return core.ErrRecoveryRequired
		}
		attrs := maps.Clone(event.Attributes)
		delete(attrs, "repository_owner")
		if event.Type != "git-push-observed" && !reflect.DeepEqual(attrs, requested.Attributes) {
			return core.ErrRecoveryRequired
		}
		switch event.Type {
		case pushStarted:
			if status.Dispatched || status.Completed != nil || !ValidID(event.Attributes["repository_owner"]) || !core.ValidEnvironmentInstanceID(event.EnvironmentInstance) {
				return core.ErrRecoveryRequired
			}
			started, status.Dispatched = event, true
		case pushConfirmed:
			if !status.Dispatched || status.State == "confirmed" || status.Completed != nil || !reflect.DeepEqual(event.Attributes, started.Attributes) {
				return core.ErrRecoveryRequired
			}
			status.State = "confirmed"
		case "completed":
			if status.Completed != nil || event.Success == nil {
				return core.ErrRecoveryRequired
			}
			status.Completed = event.Success
		}
		return nil
	})
	if err != nil {
		return PushStatus{}, core.CapabilityAuditEvent{}, err
	}
	if status.Found {
		b.mu.Lock()
		status.Active = b.active[requested.Attributes["operation_id"]]
		b.mu.Unlock()
	}
	return status, started, nil
}

// ReconcilePush makes a fresh, exact-ref fetch request. It cannot replay a
// write, restore an approval, or connect a historical name to a new owner.
func (b *Broker) ReconcilePush(ctx context.Context, environment, requestID string) (PushStatus, error) {
	status, started, err := b.pushStatus(ctx, environment, requestID)
	if err != nil {
		return PushStatus{}, err
	}
	if !status.Found || !status.Dispatched || status.Active || b.PushAudit == nil {
		return status, core.ErrRecoveryRequired
	}
	b.mu.Lock()
	server, ok := b.servers[environment]
	b.mu.Unlock()
	if !ok {
		return status, core.ErrCapabilityStale
	}
	bound := server.binding
	identities, ok := b.Environments.(interface {
		EnvironmentInstance(context.Context, core.Environment) (string, error)
	})
	if !ok {
		return status, core.ErrCapabilityStale
	}
	identity, err := identities.EnvironmentInstance(ctx, bound.Environment)
	if err != nil || identity != started.EnvironmentInstance {
		return status, core.ErrCapabilityStale
	}
	var repo Object
	for _, candidate := range bound.repositories() {
		if candidate.ID == status.Repository && candidate.Owner == started.Attributes["repository_owner"] && candidate.Remote == started.Resource {
			repo = candidate
		}
	}
	if repo.ID == "" {
		return status, core.ErrCapabilityStale
	}
	proposal := Proposal{Environment: environment, Repository: repo.ID, Remote: repo.Remote, Ref: status.Ref, Operation: "fetch"}
	_, err = b.perform(ctx, bound, proposal, func(ctx context.Context) (Response, error) {
		// Recheck the historical generation after the new approval wait too.
		current, identityErr := identities.EnvironmentInstance(ctx, bound.Environment)
		if identityErr != nil || current != started.EnvironmentInstance {
			return Response{}, core.ErrCapabilityStale
		}
		response, err := b.Repositories.RunGit(ctx, repo, AgentRequest{Operation: "observe", Repository: repo.ID, Remote: repo.Remote, Branch: repo.Branch, Ref: status.Ref})
		if err != nil {
			return Response{}, err
		}
		if response.Ref != status.Ref || !ValidOID(response.OID) || response.Error != "" || len(response.Pack) != 0 || len(response.Heads) != 0 {
			return Response{}, core.ErrRecoveryRequired
		}
		observed := started
		observed.Time, observed.Type = time.Now().UTC(), "git-push-observed"
		observed.Attributes = maps.Clone(started.Attributes)
		observed.Attributes["observed_oid"] = response.OID
		observed.Attributes["observation_request_id"] = capabilityapp.ExecutionRequestID(ctx)
		if err := b.PushAudit.Record(ctx, observed); err != nil {
			return Response{}, errors.Join(core.ErrAuditIncomplete, err)
		}
		status.ObservedOID, status.ObservedAt = response.OID, observed.Time
		switch response.OID {
		case status.NewOID:
			status.Observation = "matches-new"
		case ZeroOID:
			status.Observation = "absent"
		case status.OldOID:
			status.Observation = "matches-old"
		default:
			status.Observation = "diverged"
		}
		return response, nil
	})
	return status, err
}
