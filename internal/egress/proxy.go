package egress

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const proxyPort = 3128
const maxUnixSocketPath = 100

func (s *Service) Serve(ctx context.Context, environment core.Environment, ready func(string) error) error {
	if s == nil || s.capabilities == nil || ready == nil {
		return core.ErrInvalidArgument
	}
	socketPath, err := s.socketPath(environment)
	if err != nil {
		return err
	}
	unlock, err := lockFile(socketPath + ".lock")
	if err != nil {
		return err
	}
	defer unlock()
	listener, cleanup, err := listenUnix(socketPath)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := ready(socketPath); err != nil {
		return err
	}

	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.serveHTTP(environment.Name, w, r) }), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 64 << 10}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = server.Close()
		case <-done:
		}
	}()
	err = server.Serve(listener)
	close(done)
	if errors.Is(err, http.ErrServerClosed) || ctx.Err() != nil {
		return nil
	}
	return err
}

func (s *Service) socketPath(environment core.Environment) (string, error) {
	if !filepath.IsAbs(s.socketDir) || filepath.Clean(s.socketDir) != s.socketDir {
		return "", core.ErrInvalidArgument
	}
	name, err := stableSocketName(environment)
	if err != nil {
		return "", err
	}
	path := filepath.Join(s.socketDir, name+".sock")
	if len(path) > maxUnixSocketPath {
		return "", fmt.Errorf("egress socket path too long: %w", core.ErrInvalidArgument)
	}
	return path, nil
}

func listenUnix(path string) (net.Listener, func(), error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, nil, err
	}
	_ = os.Chmod(dir, 0o700)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeSocket == 0 {
			return nil, nil, fmt.Errorf("untrusted egress socket path: %w", core.ErrIncompatibleState)
		}
		if err := os.Remove(path); err != nil {
			return nil, nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, nil, err
	}
	created, statErr := os.Lstat(path)
	if statErr != nil {
		_ = listener.Close()
		return nil, nil, statErr
	}
	cleanup := func() {
		_ = listener.Close()
		if current, err := os.Lstat(path); err == nil && os.SameFile(created, current) {
			_ = os.Remove(path)
		}
	}
	return listener, cleanup, nil
}

func (s *Service) serveHTTP(environment string, w http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		s.serveConnect(environment, w, req)
		return
	}
	host, port, err := httpTarget(req.URL, req.Host)
	if err != nil {
		http.Error(w, "invalid proxy target", http.StatusBadRequest)
		return
	}
	endpoint, err := s.authorize(req.Context(), environment, "http", host, port)
	if err != nil {
		writeProxyError(w, err)
		return
	}

	outbound := req.Clone(req.Context())
	outbound.RequestURI = ""
	stripHopHeaders(outbound.Header)
	transport := &http.Transport{Proxy: nil, DisableKeepAlives: true, DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
		return s.dialer.DialContext(ctx, network, endpoint)
	}}
	response, err := transport.RoundTrip(outbound)
	if err != nil {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	stripHopHeaders(response.Header)
	for key, values := range response.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (s *Service) serveConnect(environment string, w http.ResponseWriter, req *http.Request) {
	host, port, err := parseAuthority(req.Host, 443)
	if err != nil {
		http.Error(w, "invalid CONNECT target", http.StatusBadRequest)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "CONNECT unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer client.Close()
	_, _ = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
	if err := buffered.Flush(); err != nil {
		return
	}
	_ = client.SetReadDeadline(time.Now().Add(10 * time.Second))
	hello, sni, ech, err := readClientHello(buffered.Reader)
	_ = client.SetReadDeadline(time.Time{})
	if err != nil || ech || sni != host {
		return
	}
	endpoint, err := s.authorize(req.Context(), environment, "https", host, port)
	if err != nil {
		return
	}
	upstream, err := s.dialer.DialContext(req.Context(), "tcp", endpoint)
	if err != nil {
		return
	}
	defer upstream.Close()
	if _, err := upstream.Write(hello); err != nil {
		return
	}
	tunnel(client, buffered.Reader, upstream)
}

func tunnel(client net.Conn, clientReader *bufio.Reader, upstream net.Conn) {
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, clientReader); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

func writeProxyError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	if isPolicyError(err) {
		status = http.StatusForbidden
	}
	if errors.Is(err, core.ErrInvalidArgument) {
		status = http.StatusBadRequest
	}
	http.Error(w, http.StatusText(status), status)
}

func stripHopHeaders(header http.Header) {
	for _, token := range strings.Split(header.Get("Connection"), ",") {
		if name := strings.TrimSpace(token); name != "" {
			header.Del(name)
		}
	}
	for _, name := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		header.Del(name)
	}
}
