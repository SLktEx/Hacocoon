package capability

import (
	"context"
	"fmt"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// OfficialBuildPolicy layers one short-lived, controller-owned package-network
// grant over the operator policy. The grant is bound to an exact temporary Base
// Builder Environment name and a fixed set of package hosts. It is never saved
// to policy.json and disappears before the temporary Environment is deleted.
type OfficialBuildPolicy struct {
	*FilePolicyEvaluator
	mu     sync.RWMutex
	grants map[string]map[string]struct{}
}

func NewOfficialBuildPolicy(file *FilePolicyEvaluator) *OfficialBuildPolicy {
	return &OfficialBuildPolicy{FilePolicyEvaluator: file, grants: map[string]map[string]struct{}{}}
}

func (p *OfficialBuildPolicy) AcquireOfficialBuild(_ context.Context, environment string, hosts []string) (func(), error) {
	if p == nil || p.FilePolicyEvaluator == nil || environment == "" || len(hosts) == 0 {
		return nil, core.ErrInvalidArgument
	}
	allowed := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		if host == "" {
			return nil, core.ErrInvalidArgument
		}
		allowed[host] = struct{}{}
	}
	p.mu.Lock()
	if _, exists := p.grants[environment]; exists {
		p.mu.Unlock()
		return nil, core.ErrAlreadyExists
	}
	p.grants[environment] = allowed
	p.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			p.mu.Lock()
			delete(p.grants, environment)
			p.mu.Unlock()
		})
	}, nil
}

func (p *OfficialBuildPolicy) Evaluate(ctx context.Context, req core.CapabilityRequest) (core.PolicyEvaluation, error) {
	if p == nil || p.FilePolicyEvaluator == nil {
		return core.PolicyEvaluation{}, fmt.Errorf("official Base build policy unavailable: %w", core.ErrPolicyDenied)
	}
	if p.allowed(req) {
		return core.PolicyEvaluation{Decision: core.PolicyAllow, Reason: "trusted official Base Builder package access"}, nil
	}
	return p.FilePolicyEvaluator.Evaluate(ctx, req)
}

func (p *OfficialBuildPolicy) allowed(req core.CapabilityRequest) bool {
	if req.Environment == "" || !core.ValidEnvironmentInstanceID(req.EnvironmentInstance) {
		return false
	}
	p.mu.RLock()
	hosts := p.grants[req.Environment]
	_, hostAllowed := hosts[req.Resource]
	p.mu.RUnlock()
	if !hostAllowed {
		return false
	}
	switch req.Capability {
	case "network.resolve":
		return req.Action == "lookup" && len(req.Attributes) == 0
	case "network.egress":
		if req.Action != "connect" || len(req.Attributes) != 2 {
			return false
		}
		protocol, port := req.Attributes["protocol"], req.Attributes["port"]
		return (protocol == "http" && port == "80") || (protocol == "https" && port == "443")
	default:
		return false
	}
}
