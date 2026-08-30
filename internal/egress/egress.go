package egress

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/logging"
)

const (
	Capability    = "network.egress"
	ActionConnect = "connect"
)

type capabilityRequester interface {
	Request(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)
}

type Broker struct {
	capabilities capabilityRequester
}

func NewBroker(capabilities capabilityRequester) *Broker {
	return &Broker{capabilities: capabilities}
}

// Authorize sends one normalized hostname-scoped connection through the
// existing Policy / Approval / Capability boundary. It intentionally returns a
// connection-scoped grant instead of a reusable IP authorization.
func (b *Broker) Authorize(ctx context.Context, request core.EgressRequest) (grant core.EgressGrant, err error) {
	started := time.Now()
	ctx = logging.With(ctx, "operation", "authorize_egress", "environment_id", request.Environment)
	logger := logging.FromContext(ctx).With("component", "network")
	defer func() {
		if err != nil {
			logger.DebugContext(ctx, "egress authorization failed",
				"duration_ms", time.Since(started).Milliseconds(),
				"error", err,
			)
			return
		}
		logger.DebugContext(ctx, "egress authorized",
			"duration_ms", time.Since(started).Milliseconds(),
			"request_id", grant.RequestID,
		)
	}()

	if b == nil || b.capabilities == nil {
		return core.EgressGrant{}, fmt.Errorf("egress capability boundary unavailable: %w", core.ErrPolicyDenied)
	}
	normalized, err := NormalizeRequest(request)
	if err != nil {
		return core.EgressGrant{}, err
	}
	logger.DebugContext(ctx, "authorizing egress",
		"protocol", normalized.Protocol,
		"target_host", normalized.Host,
		"target_port", normalized.Port,
	)
	result, err := b.capabilities.Request(ctx, core.CapabilityRequest{
		Capability:  Capability,
		Action:      ActionConnect,
		Resource:    normalized.Host,
		Environment: normalized.Environment,
		Attributes: map[string]string{
			"protocol": string(normalized.Protocol),
			"port":     strconv.Itoa(normalized.Port),
		},
	})
	if err != nil {
		return core.EgressGrant{}, err
	}
	return core.EgressGrant{
		RequestID:   result.RequestID,
		Environment: normalized.Environment,
		Host:        normalized.Host,
		Port:        normalized.Port,
		Protocol:    normalized.Protocol,
	}, nil
}

// Provider is the execution half of the Capability boundary. Execution here
// means granting exactly the normalized connection described by the request;
// actual network dialing remains in the Standard proxy.
type Provider struct{}

func (Provider) Capability() string { return Capability }

func (Provider) Execute(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	if req.Capability != Capability || req.Action != ActionConnect || len(req.Parameters) != 0 {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	if len(req.Attributes) != 2 {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	port, err := strconv.Atoi(req.Attributes["port"])
	if err != nil {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	normalized, err := NormalizeRequest(core.EgressRequest{
		Environment: req.Environment,
		Host:        req.Resource,
		Port:        port,
		Protocol:    core.EgressProtocol(req.Attributes["protocol"]),
	})
	if err != nil {
		return core.CapabilityResult{}, err
	}
	if normalized.Environment != req.Environment || normalized.Host != req.Resource || strconv.Itoa(normalized.Port) != req.Attributes["port"] || string(normalized.Protocol) != req.Attributes["protocol"] {
		return core.CapabilityResult{}, fmt.Errorf("egress authority must be canonical before capability execution: %w", core.ErrInvalidArgument)
	}
	return core.CapabilityResult{Provider: "standard.egress.authorization"}, nil
}

func NormalizeRequest(request core.EgressRequest) (core.EgressRequest, error) {
	if strings.TrimSpace(request.Environment) == "" || strings.TrimSpace(request.Environment) != request.Environment || strings.ContainsAny(request.Environment, "\r\n\x00") {
		return core.EgressRequest{}, core.ErrInvalidArgument
	}
	host, err := CanonicalHost(request.Host)
	if err != nil {
		return core.EgressRequest{}, err
	}
	if request.Port < 1 || request.Port > 65535 {
		return core.EgressRequest{}, core.ErrInvalidArgument
	}
	switch request.Protocol {
	case core.EgressHTTP, core.EgressHTTPS:
	default:
		return core.EgressRequest{}, core.ErrUnsupported
	}
	request.Host = host
	return request, nil
}

// CanonicalHost accepts DNS hostnames only. IP literals are deliberately
// rejected so a sandbox cannot bypass hostname policy by addressing a resolved
// endpoint directly. A trailing DNS root dot and ASCII case are normalized.
func CanonicalHost(raw string) (string, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || strings.ContainsAny(raw, "\r\n\x00/@%") {
		return "", core.ErrInvalidArgument
	}
	host := strings.ToLower(raw)
	if strings.HasSuffix(host, ".") {
		host = strings.TrimSuffix(host, ".")
	}
	if host == "" || len(host) > 253 || net.ParseIP(strings.Trim(host, "[]")) != nil || strings.Contains(host, ":") {
		return "", core.ErrInvalidArgument
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", core.ErrInvalidArgument
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return "", core.ErrInvalidArgument
		}
	}
	return host, nil
}
