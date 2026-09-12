package egressproxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

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
	addresses, ok := p.prepareUpstream(w, r, core.EgressRequest{Environment: environment, Host: host, Port: port, Protocol: core.EgressHTTP})
	if !ok {
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
