// Package networkrelay implements bounded development connections on the
// existing guarded Standard endpoint. It never serves management authority.
package networkrelay

import (
	"context"
	"net"
	"net/netip"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	Capability      = "network.connect"
	Action          = "connect"
	Path            = "/_haco/operations/network/connect"
	MaxDuration     = 3600
	MaxUDPDurations = 300
	MaxDatagram     = 65507
)

type Spec struct {
	Kind            string `json:"kind"`
	Target          string `json:"target"`
	Protocol        string `json:"protocol"`
	Port            int    `json:"port,omitempty"`
	DurationSeconds int    `json:"duration_seconds"`
}

type Source struct {
	Environment string `json:"environment"`
	Instance    string `json:"instance"`
}

// Target is trusted, immutable preparation evidence, never a guest-supplied
// address override. Owner binds an Env generation or a Host service registration.
type Target struct {
	Kind      string       `json:"kind"`
	Name      string       `json:"name"`
	Protocol  string       `json:"protocol"`
	Port      int          `json:"port"`
	Addresses []netip.Addr `json:"addresses"`
	Owner     string       `json:"owner,omitempty"`
}

type Session struct {
	ID             string    `json:"id"`
	RequestID      string    `json:"request_id"`
	Source         Source    `json:"source"`
	Target         Target    `json:"target"`
	Peer           string    `json:"peer,omitempty"`
	PolicyRevision string    `json:"policy_revision"`
	Phase          string    `json:"phase"`
	State          string    `json:"state"`
	Reason         string    `json:"reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type Requester interface {
	Request(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)
}
type SourceResolver interface {
	ResolveEnvironmentInstance(context.Context, net.IP) (string, string, error)
}
type Targets interface {
	Resolve(context.Context, Source, Spec) (Target, error)
	Verify(context.Context, Target) error
}
type Authority interface {
	CurrentInstance(context.Context, string) (string, error)
	PolicyRevision(context.Context) (string, error)
	Evaluate(context.Context, core.CapabilityRequest) (core.PolicyEvaluation, error)
	Record(context.Context, core.CapabilityAuditEvent) error
}
