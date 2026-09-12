package egressproxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	egressapp "github.com/SLktEx/Hacocoon/internal/egress"
)

const clientHelloTimeout = 10 * time.Second

func (p *Proxy) handleConnect(w http.ResponseWriter, r *http.Request, environment string) {
	host, port, err := parseAuthority(r.Host, 443)
	if err != nil {
		http.Error(w, "invalid CONNECT authority", http.StatusBadRequest)
		return
	}
	addresses, ok := p.prepareUpstream(w, r, core.EgressRequest{Environment: environment, Host: host, Port: port, Protocol: core.EgressHTTPS})
	if !ok {
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
	if owner, ok := r.Context().Value(proxyConnectionsKey{}).(*proxyListener); ok {
		upstream, err = owner.trackUpstream(upstream)
		if err != nil {
			return
		}
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

func closeWrite(conn net.Conn) {
	if tcp, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = tcp.CloseWrite()
	}
}
