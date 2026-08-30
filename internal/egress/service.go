package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const Capability = "network.egress"
const connectAction = "connect"

type resolver interface {
	LookupNetIP(context.Context, string, string) ([]netip.Addr, error)
}

type Provider struct{ resolver resolver }

func NewProvider() *Provider                       { return &Provider{resolver: net.DefaultResolver} }
func newProviderWithResolver(r resolver) *Provider { return &Provider{resolver: r} }
func (*Provider) Capability() string               { return Capability }

func (p *Provider) Execute(ctx context.Context, req core.CapabilityRequest) (core.CapabilityResult, error) {
	if p == nil || p.resolver == nil || req.Capability != Capability || req.Action != connectAction || strings.TrimSpace(req.Environment) == "" {
		return core.CapabilityResult{}, core.ErrInvalidArgument
	}
	host, port, _, err := targetFromRequest(req)
	if err != nil {
		return core.CapabilityResult{}, err
	}
	addresses, err := p.resolver.LookupNetIP(ctx, "ip", host+".")
	if err != nil {
		return core.CapabilityResult{}, fmt.Errorf("resolve approved egress hostname: %w", err)
	}
	if len(addresses) == 0 {
		return core.CapabilityResult{}, fmt.Errorf("approved egress hostname has no addresses: %w", core.ErrIncompatibleState)
	}
	safe := make([]netip.Addr, 0, len(addresses))
	for _, addr := range addresses {
		addr = addr.Unmap()
		if !publicAddress(addr) {
			return core.CapabilityResult{}, fmt.Errorf("approved hostname resolved to non-public address %s: %w", addr, core.ErrPolicyDenied)
		}
		safe = append(safe, addr)
	}
	sort.Slice(safe, func(i, j int) bool { return safe[i].Compare(safe[j]) < 0 })
	endpoint := net.JoinHostPort(safe[0].String(), strconv.Itoa(port))
	return core.CapabilityResult{Provider: Capability, Output: endpoint}, nil
}

func targetFromRequest(req core.CapabilityRequest) (string, int, string, error) {
	if len(req.Parameters) != 0 {
		return "", 0, "", core.ErrInvalidArgument
	}
	if len(req.Attributes) != 3 {
		return "", 0, "", core.ErrInvalidArgument
	}
	host, err := canonicalHostname(req.Attributes["hostname"])
	if err != nil {
		return "", 0, "", err
	}
	protocol := strings.ToLower(strings.TrimSpace(req.Attributes["protocol"]))
	port, err := strconv.Atoi(strings.TrimSpace(req.Attributes["port"]))
	if err != nil || port < 1 || port > 65535 {
		return "", 0, "", core.ErrInvalidArgument
	}
	if protocol != "http" && protocol != "https" {
		return "", 0, "", core.ErrUnsupported
	}
	want := targetResource(protocol, host, port)
	if req.Resource != want {
		return "", 0, "", fmt.Errorf("egress target changed after policy evaluation: %w", core.ErrIncompatibleState)
	}
	return host, port, protocol, nil
}

func targetResource(protocol, host string, port int) string {
	return protocol + "://" + host + ":" + strconv.Itoa(port)
}

func canonicalHostname(raw string) (string, error) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if host == "" || len(host) > 253 || !strings.Contains(host, ".") || net.ParseIP(host) != nil {
		return "", core.ErrInvalidArgument
	}
	for _, r := range host {
		if r > 127 {
			return "", core.ErrInvalidArgument
		}
	}
	labels := strings.Split(host, ".")
	for _, label := range labels {
		if len(label) < 1 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", core.ErrInvalidArgument
		}
		for _, c := range label {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
				return "", core.ErrInvalidArgument
			}
		}
	}
	return host, nil
}

var blockedPrefixes = mustPrefixes(
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
	"172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16", "198.18.0.0/15",
	"198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
	"::/128", "::1/128", "2001:db8::/32", "fc00::/7", "fe80::/10", "ff00::/8",
)

func mustPrefixes(values ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		out = append(out, netip.MustParsePrefix(value))
	}
	return out
}
func publicAddress(addr netip.Addr) bool {
	if !addr.IsValid() || !addr.IsGlobalUnicast() {
		return false
	}
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

type requester interface {
	Request(context.Context, core.CapabilityRequest) (core.CapabilityResult, error)
}

type Service struct {
	capabilities requester
	socketDir    string
	approvalMu   sync.Mutex
	dialer       net.Dialer
}

func New(capabilities requester, socketDir string) *Service {
	return &Service{capabilities: capabilities, socketDir: socketDir}
}

func (s *Service) authorize(ctx context.Context, environment, protocol, rawHost string, port int) (string, error) {
	if s == nil || s.capabilities == nil || strings.TrimSpace(environment) == "" {
		return "", core.ErrInvalidArgument
	}
	host, err := canonicalHostname(rawHost)
	if err != nil {
		return "", err
	}
	resource := targetResource(protocol, host, port)
	s.approvalMu.Lock()
	result, err := s.capabilities.Request(ctx, core.CapabilityRequest{
		Capability: Capability, Action: connectAction, Resource: resource, Environment: environment,
		Attributes: map[string]string{"hostname": host, "port": strconv.Itoa(port), "protocol": protocol},
	})
	s.approvalMu.Unlock()
	if err != nil {
		return "", err
	}
	endpoint := strings.TrimSpace(result.Output)
	endpointHost, endpointPort, splitErr := net.SplitHostPort(endpoint)
	if splitErr != nil || endpointPort != strconv.Itoa(port) || net.ParseIP(endpointHost) == nil {
		return "", fmt.Errorf("egress provider returned invalid pinned endpoint: %w", core.ErrIncompatibleState)
	}
	return endpoint, nil
}

func parseAuthority(raw string, defaultPort int) (string, int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "/?#@") {
		return "", 0, core.ErrInvalidArgument
	}
	host := raw
	port := defaultPort
	if strings.Contains(raw, ":") {
		h, p, err := net.SplitHostPort(raw)
		if err != nil {
			return "", 0, core.ErrInvalidArgument
		}
		parsed, err := strconv.Atoi(p)
		if err != nil || parsed < 1 || parsed > 65535 {
			return "", 0, core.ErrInvalidArgument
		}
		host, port = h, parsed
	}
	host, err := canonicalHostname(host)
	if err != nil {
		return "", 0, err
	}
	return host, port, nil
}

func httpTarget(reqURL *url.URL, hostHeader string) (string, int, error) {
	if reqURL == nil || !reqURL.IsAbs() || strings.ToLower(reqURL.Scheme) != "http" || reqURL.User != nil || reqURL.Host == "" {
		return "", 0, core.ErrInvalidArgument
	}
	host, port, err := parseAuthority(reqURL.Host, 80)
	if err != nil {
		return "", 0, err
	}
	hh, hp, err := parseAuthority(hostHeader, 80)
	if err != nil || hh != host || hp != port {
		return "", 0, core.ErrInvalidArgument
	}
	return host, port, nil
}

func isPolicyError(err error) bool {
	return errors.Is(err, core.ErrPolicyDenied) || errors.Is(err, core.ErrApprovalDenied)
}
