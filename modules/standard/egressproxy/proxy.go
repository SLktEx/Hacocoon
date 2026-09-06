package egressproxy

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	egressapp "github.com/SLktEx/Hacocoon/internal/egress"
)

const (
	DefaultPort         = 18080
	maxClientHelloBytes = 128 << 10
	clientHelloTimeout  = 10 * time.Second
)

type Authorizer interface {
	Authorize(context.Context, core.EgressRequest) (core.EgressGrant, error)
}

type SourceResolver interface {
	ResolveEnvironment(context.Context, net.IP) (string, error)
}

type DNSResolver interface {
	LookupIPAddr(context.Context, string) ([]net.IPAddr, error)
}

type Proxy struct {
	authorizer Authorizer
	sources    SourceResolver
	resolver   DNSResolver
	dial       func(context.Context, string, string) (net.Conn, error)
}

func New(authorizer Authorizer, sources SourceResolver) *Proxy {
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	return &Proxy{
		authorizer: authorizer,
		sources:    sources,
		resolver:   net.DefaultResolver,
		dial:       dialer.DialContext,
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p == nil || p.authorizer == nil || p.sources == nil || p.resolver == nil || p.dial == nil {
		http.Error(w, "egress proxy unavailable", http.StatusServiceUnavailable)
		return
	}
	environment, err := p.resolveSource(r.Context(), r.RemoteAddr)
	if err != nil {
		http.Error(w, "unmanaged egress source", http.StatusForbidden)
		return
	}
	if r.Method == http.MethodConnect {
		p.handleConnect(w, r, environment)
		return
	}
	p.handleHTTP(w, r, environment)
}

func (p *Proxy) resolveSource(ctx context.Context, remote string) (string, error) {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return "", core.ErrInvalidArgument
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		return "", core.ErrPolicyDenied
	}
	return p.sources.ResolveEnvironment(ctx, ip)
}

func (p *Proxy) handleHTTP(w http.ResponseWriter, r *http.Request, environment string) {
	if r.URL == nil || !r.URL.IsAbs() || !strings.EqualFold(r.URL.Scheme, "http") || r.URL.User != nil {
		http.Error(w, "absolute http proxy URL required", http.StatusBadRequest)
		return
	}
	host, port, err := parseAuthority(r.URL.Host, 80)
	if err != nil {
		http.Error(w, "invalid upstream authority", http.StatusBadRequest)
		return
	}
	if r.Host != "" {
		hostHeader, hostPort, hostErr := parseAuthority(r.Host, 80)
		if hostErr != nil || hostHeader != host || hostPort != port {
			http.Error(w, "Host and proxy target differ", http.StatusBadRequest)
			return
		}
	}
	if _, err := p.authorizer.Authorize(r.Context(), core.EgressRequest{Environment: environment, Host: host, Port: port, Protocol: core.EgressHTTP}); err != nil {
		http.Error(w, "egress denied", http.StatusForbidden)
		return
	}
	addresses, err := p.resolvePinned(r.Context(), host)
	if err != nil {
		http.Error(w, "upstream resolution denied", http.StatusBadGateway)
		return
	}

	transport := &http.Transport{
		Proxy:             nil,
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			requestedHost, requestedPort, splitErr := net.SplitHostPort(address)
			if splitErr != nil || !strings.EqualFold(requestedHost, host) || requestedPort != strconv.Itoa(port) {
				return nil, core.ErrPolicyDenied
			}
			return p.dialPinned(ctx, addresses, port)
		},
	}
	defer transport.CloseIdleConnections()

	upstream := r.Clone(r.Context())
	upstream.RequestURI = ""
	upstream.URL.Scheme = "http"
	upstream.URL.Host = net.JoinHostPort(host, strconv.Itoa(port))
	if port == 80 {
		upstream.URL.Host = host
	}
	removeHopHeaders(upstream.Header)
	upstream.Header.Del("Proxy-Authorization")
	upstream.Header.Del("Proxy-Connection")
	response, err := transport.RoundTrip(upstream)
	if err != nil {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	removeHopHeaders(response.Header)
	copyHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request, environment string) {
	host, port, err := parseAuthority(r.Host, 443)
	if err != nil {
		http.Error(w, "invalid CONNECT authority", http.StatusBadRequest)
		return
	}
	if _, err := p.authorizer.Authorize(r.Context(), core.EgressRequest{Environment: environment, Host: host, Port: port, Protocol: core.EgressHTTPS}); err != nil {
		http.Error(w, "egress denied", http.StatusForbidden)
		return
	}
	addresses, err := p.resolvePinned(r.Context(), host)
	if err != nil {
		http.Error(w, "upstream resolution denied", http.StatusBadGateway)
		return
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "CONNECT unsupported", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	stopClient := context.AfterFunc(r.Context(), func() { _ = client.Close() })
	defer stopClient()
	if buffered.Reader.Buffered() != 0 {
		// A pipelined ClientHello would bypass the bounded SNI reader below.
		return
	}
	if _, err := buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err := buffered.Flush(); err != nil {
		return
	}
	_ = client.SetReadDeadline(time.Now().Add(clientHelloTimeout))
	prefix, sni, err := readClientHelloServerName(client)
	if err != nil {
		return
	}
	_ = client.SetReadDeadline(time.Time{})
	canonicalSNI, err := egressapp.CanonicalHost(sni)
	if err != nil || canonicalSNI != host {
		return
	}

	upstream, err := p.dialPinned(r.Context(), addresses, port)
	if err != nil {
		return
	}
	defer upstream.Close()
	stopUpstream := context.AfterFunc(r.Context(), func() { _ = upstream.Close() })
	defer stopUpstream()
	if _, err := upstream.Write(prefix); err != nil {
		return
	}

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, client); closeWrite(upstream); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); closeWrite(client); done <- struct{}{} }()
	<-done
	<-done
}

func parseAuthority(authority string, defaultPort int) (string, int, error) {
	if authority == "" || strings.ContainsAny(authority, "\r\n\x00/@") {
		return "", 0, core.ErrInvalidArgument
	}
	host := authority
	port := defaultPort
	if parsedHost, parsedPort, err := net.SplitHostPort(authority); err == nil {
		host = parsedHost
		value, convErr := strconv.Atoi(parsedPort)
		if convErr != nil || value < 1 || value > 65535 {
			return "", 0, core.ErrInvalidArgument
		}
		port = value
	} else if strings.Contains(authority, ":") {
		return "", 0, core.ErrInvalidArgument
	}
	canonical, err := egressapp.CanonicalHost(host)
	if err != nil {
		return "", 0, err
	}
	return canonical, port, nil
}

func (p *Proxy) resolvePinned(ctx context.Context, host string) ([]netip.Addr, error) {
	resolved, err := p.resolver.LookupIPAddr(ctx, host)
	if err != nil || len(resolved) == 0 {
		return nil, fmt.Errorf("resolve %s: %w", host, err)
	}
	addresses := make([]netip.Addr, 0, len(resolved))
	seen := map[netip.Addr]struct{}{}
	for _, item := range resolved {
		addr, ok := netip.AddrFromSlice(item.IP)
		if !ok {
			return nil, core.ErrPolicyDenied
		}
		addr = addr.Unmap()
		if !publicDialAddress(addr) {
			// Reject the whole answer set. Silently dropping a private answer would
			// make mixed/rebinding responses dependent on resolver ordering.
			return nil, core.ErrPolicyDenied
		}
		if _, exists := seen[addr]; exists {
			continue
		}
		seen[addr] = struct{}{}
		addresses = append(addresses, addr)
	}
	if len(addresses) == 0 {
		return nil, core.ErrPolicyDenied
	}
	return addresses, nil
}

func (p *Proxy) dialPinned(ctx context.Context, addresses []netip.Addr, port int) (net.Conn, error) {
	var errs []error
	for _, address := range addresses {
		conn, err := p.dial(ctx, "tcp", net.JoinHostPort(address.String(), strconv.Itoa(port)))
		if err == nil {
			return conn, nil
		}
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return nil, core.ErrRuntimeUnavailable
	}
	return nil, errors.Join(errs...)
}

func publicDialAddress(addr netip.Addr) bool {
	if !addr.IsValid() || !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, prefix := range forbiddenDialPrefixes {
		if prefix.Contains(addr) {
			return false
		}
	}
	return true
}

var forbiddenDialPrefixes = mustPrefixes(
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
	"2001:db8::/32",
)

func mustPrefixes(values ...string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		result = append(result, netip.MustParsePrefix(value))
	}
	return result
}

func readClientHelloServerName(reader io.Reader) ([]byte, string, error) {
	var raw []byte
	var handshake []byte
	for len(raw) < maxClientHelloBytes {
		header := make([]byte, 5)
		if _, err := io.ReadFull(reader, header); err != nil {
			return nil, "", err
		}
		if header[0] != 22 {
			return nil, "", core.ErrPolicyDenied
		}
		length := int(header[3])<<8 | int(header[4])
		if length < 1 || len(raw)+5+length > maxClientHelloBytes {
			return nil, "", core.ErrPolicyDenied
		}
		payload := make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, "", err
		}
		raw = append(raw, header...)
		raw = append(raw, payload...)
		handshake = append(handshake, payload...)
		if len(handshake) < 4 {
			continue
		}
		if handshake[0] != 1 {
			return nil, "", core.ErrPolicyDenied
		}
		messageLen := int(handshake[1])<<16 | int(handshake[2])<<8 | int(handshake[3])
		if messageLen < 1 || messageLen+4 > maxClientHelloBytes {
			return nil, "", core.ErrPolicyDenied
		}
		if len(handshake) < messageLen+4 {
			continue
		}
		name, err := parseClientHelloServerName(handshake[4 : messageLen+4])
		return raw, name, err
	}
	return nil, "", core.ErrPolicyDenied
}

func parseClientHelloServerName(body []byte) (string, error) {
	// legacy_version(2), random(32), session id, cipher suites,
	// compression methods, then extensions.
	if len(body) < 35 {
		return "", core.ErrPolicyDenied
	}
	pos := 34
	sessionLen := int(body[pos])
	pos++
	if pos+sessionLen+2 > len(body) {
		return "", core.ErrPolicyDenied
	}
	pos += sessionLen
	cipherLen := int(body[pos])<<8 | int(body[pos+1])
	pos += 2
	if cipherLen < 2 || pos+cipherLen+1 > len(body) {
		return "", core.ErrPolicyDenied
	}
	pos += cipherLen
	compressionLen := int(body[pos])
	pos++
	if pos+compressionLen == len(body) {
		return "", core.ErrPolicyDenied
	}
	if pos+compressionLen+2 > len(body) {
		return "", core.ErrPolicyDenied
	}
	pos += compressionLen
	extensionsLen := int(body[pos])<<8 | int(body[pos+1])
	pos += 2
	if extensionsLen < 0 || pos+extensionsLen != len(body) {
		return "", core.ErrPolicyDenied
	}
	end := pos + extensionsLen
	for pos+4 <= end {
		typeID := int(body[pos])<<8 | int(body[pos+1])
		length := int(body[pos+2])<<8 | int(body[pos+3])
		pos += 4
		if pos+length > end {
			return "", core.ErrPolicyDenied
		}
		if typeID == 0 {
			return parseServerNameExtension(body[pos : pos+length])
		}
		pos += length
	}
	return "", core.ErrPolicyDenied
}

func parseServerNameExtension(value []byte) (string, error) {
	if len(value) < 2 {
		return "", core.ErrPolicyDenied
	}
	listLen := int(value[0])<<8 | int(value[1])
	if listLen+2 != len(value) {
		return "", core.ErrPolicyDenied
	}
	pos := 2
	for pos+3 <= len(value) {
		nameType := value[pos]
		nameLen := int(value[pos+1])<<8 | int(value[pos+2])
		pos += 3
		if nameLen < 1 || pos+nameLen > len(value) {
			return "", core.ErrPolicyDenied
		}
		if nameType == 0 {
			return string(value[pos : pos+nameLen]), nil
		}
		pos += nameLen
	}
	return "", core.ErrPolicyDenied
}

func removeHopHeaders(header http.Header) {
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func closeWrite(conn net.Conn) {
	if tcp, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = tcp.CloseWrite()
	}
}

// Ensure imports cannot accidentally drift away from the canonical authority
// implementation while this package remains the Standard enforcement layer.
var _ = egressapp.Capability
var _ = bufio.ErrInvalidUnreadByte
