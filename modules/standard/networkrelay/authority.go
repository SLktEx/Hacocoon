package networkrelay

import (
	"context"
	"net/netip"
	"sort"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/capability"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/egress"
)

func validateSpec(s Spec) error {
	if s.DurationSeconds < 1 || s.DurationSeconds > MaxDuration || (s.Protocol != "tcp" && s.Protocol != "udp") {
		return core.ErrInvalidArgument
	}
	if s.Protocol == "udp" && s.DurationSeconds > MaxUDPDurations {
		return core.ErrInvalidArgument
	}
	if s.Target == "" || len(s.Target) > 253 || strings.TrimSpace(s.Target) != s.Target || strings.ContainsAny(s.Target, "\x00\r\n/@%") {
		return core.ErrInvalidArgument
	}
	switch s.Kind {
	case "external":
		if s.Port < 1 || s.Port > 65535 {
			return core.ErrInvalidArgument
		}
		if a, err := netip.ParseAddr(s.Target); err == nil {
			if a.Zone() != "" || !a.IsGlobalUnicast() || a.IsLoopback() || a.IsLinkLocalUnicast() {
				return core.ErrPolicyDenied
			}
		} else if host, err := egress.CanonicalHost(s.Target); err != nil || host != s.Target {
			return core.ErrInvalidArgument
		}
	case "host", "environment":
		if !validName(s.Target) || s.Port < 0 || s.Port > 65535 || (s.Kind == "environment" && s.Port == 0) {
			return core.ErrInvalidArgument
		}
	default:
		return core.ErrInvalidArgument
	}
	return nil
}
func validName(s string) bool {
	if len(s) < 1 || len(s) > 63 || s[0] == '-' {
		return false
	}
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}
	return true
}
func requestFor(source Source, target Target, duration int) (core.CapabilityRequest, error) {
	if !validName(source.Environment) || !core.ValidEnvironmentInstanceID(source.Instance) || duration < 1 || duration > MaxDuration ||
		(target.Protocol != "tcp" && target.Protocol != "udp") || target.Port < 1 || target.Port > 65535 || len(target.Addresses) == 0 || len(target.Addresses) > 32 {
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}
	if target.Protocol == "udp" && duration > MaxUDPDurations {
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}
	switch target.Kind {
	case "external":
		if target.Owner != "" {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
		if err := validateSpec(Spec{Kind: target.Kind, Target: target.Name, Port: target.Port, Protocol: target.Protocol, DurationSeconds: duration}); err != nil {
			return core.CapabilityRequest{}, err
		}
	case "host":
		service := capability.NetworkService{Name: target.Name, Instance: target.Owner, Protocol: target.Protocol, Port: target.Port}
		if len(target.Addresses) != 1 {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
		service.Address = target.Addresses[0].String()
		if capability.ValidateNetworkService(service) != nil {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
	case "environment":
		if !validName(target.Name) || !core.ValidEnvironmentInstanceID(target.Owner) {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
	default:
		return core.CapabilityRequest{}, core.ErrInvalidArgument
	}
	addresses := make([]string, 0, len(target.Addresses))
	seen := map[netip.Addr]bool{}
	for _, address := range target.Addresses {
		if !address.IsValid() || address.Zone() != "" || address.IsUnspecified() || address.IsMulticast() || address.IsLinkLocalUnicast() || seen[address] {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
		if address.Is4In6() {
			return core.CapabilityRequest{}, core.ErrInvalidArgument
		}
		if target.Kind == "external" && address.IsLoopback() {
			return core.CapabilityRequest{}, core.ErrPolicyDenied
		}
		seen[address] = true
		addresses = append(addresses, address.String())
	}
	sort.Strings(addresses)
	return core.CapabilityRequest{
		Capability: Capability, Action: Action, Environment: source.Environment, EnvironmentInstance: source.Instance, Resource: target.Name,
		Attributes: map[string]string{"destination_kind": target.Kind, "destination_instance": target.Owner, "protocol": target.Protocol, "port": strconv.Itoa(target.Port), "addresses": strings.Join(addresses, ","), "duration_seconds": strconv.Itoa(duration)},
	}, nil
}

// Provider approves only canonical authority. Socket creation remains in the
// Standard relay, after the Capability service completes its audit.
type Provider struct{}

func (Provider) Capability() string { return Capability }
func (Provider) Execute(_ context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	if req.Capability != Capability || req.Action != Action || len(req.Attributes) != 6 || len(req.Parameters) != 0 {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	port, err := strconv.Atoi(req.Attributes["port"])
	if err != nil {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	duration, err := strconv.Atoi(req.Attributes["duration_seconds"])
	if err != nil {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	target := Target{Kind: req.Attributes["destination_kind"], Name: req.Resource, Owner: req.Attributes["destination_instance"], Protocol: req.Attributes["protocol"], Port: port}
	for _, raw := range strings.Split(req.Attributes["addresses"], ",") {
		address, err := netip.ParseAddr(raw)
		if err != nil {
			return core.CapabilityResult{}, core.ErrInvalidArgument
		}
		target.Addresses = append(target.Addresses, address)
	}
	canonical, err := requestFor(Source{Environment: req.Environment, Instance: req.EnvironmentInstance}, target, duration)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	for key, value := range canonical.Attributes {
		if req.Attributes[key] != value {
			return core.CapabilityResult{}, core.ErrInvalidArgument
		}
	}
	return core.CapabilityResult{Provider: "standard.network.authorization"}, nil
}
