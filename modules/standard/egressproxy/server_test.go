package egressproxy

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/logging"
)

// Only the component test supplies a managed peer address. The real service
// uses the socket's peer and the persisted Incus source resolver unchanged.
type managedPeerListener struct{ net.Listener }
type managedPeerConn struct{ net.Conn }

func (c managedPeerConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("10.200.0.20"), Port: 40000}
}
func (l managedPeerListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return managedPeerConn{c}, nil
}

func clientHello(t *testing.T) []byte {
	t.Helper()
	client, peer := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = tls.Client(client, &tls.Config{ServerName: "example.com"}).Handshake()
		_ = client.Close()
	}()
	_ = peer.SetDeadline(time.Now().Add(2 * time.Second))
	hello, name, err := readClientHelloServerName(peer)
	_ = peer.Close()
	<-done
	if err != nil || name != "example.com" {
		t.Fatalf("ClientHello: %q %v", name, err)
	}
	return hello
}

func TestProxyShutdownClosesCONNECTAtEveryBlockingStage(t *testing.T) {
	for _, stage := range []string{"client-hello", "upstream-prefix", "tunnel"} {
		t.Run(stage, func(t *testing.T) {
			listener, err := net.Listen("tcp4", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			proxy := New(&fakeAuthorizer{}, fakeSources{environment: "env-a"})
			proxy.resolver = fakeDNS{addresses: []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}}
			upstreamReady := make(chan net.Conn, 1)
			proxy.dial = func(context.Context, string, string) (net.Conn, error) {
				conn, peer := net.Pipe()
				upstreamReady <- peer
				return conn, nil
			}
			done := make(chan error, 1)
			go func() { done <- proxy.Serve(ctx, managedPeerListener{listener}) }()
			client, err := net.Dial("tcp4", listener.Addr().String())
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			_ = client.SetDeadline(time.Now().Add(3 * time.Second))
			_, err = io.WriteString(client, "CONNECT example.com:443 HTTP/1.1\r\nHost: example.com:443\r\n\r\n")
			if err != nil {
				t.Fatal(err)
			}
			response, err := http.ReadResponse(bufio.NewReader(client), &http.Request{Method: http.MethodConnect})
			if err != nil || response.StatusCode != 200 {
				t.Fatalf("CONNECT: %v %v", response, err)
			}
			var upstream net.Conn
			if stage != "client-hello" {
				hello := clientHello(t)
				if _, err := client.Write(hello); err != nil {
					t.Fatal(err)
				}
				select {
				case upstream = <-upstreamReady:
				case <-time.After(2 * time.Second):
					t.Fatal("upstream was not dialed")
				}
				defer upstream.Close()
				_ = upstream.SetDeadline(time.Now().Add(3 * time.Second))
				if stage == "tunnel" {
					prefix := make([]byte, len(hello))
					if _, err := io.ReadFull(upstream, prefix); err != nil || !bytes.Equal(prefix, hello) {
						t.Fatalf("prefix: %v", err)
					}
					if _, err := client.Write([]byte("payload")); err != nil {
						t.Fatal(err)
					}
					payload := make([]byte, 7)
					if _, err := io.ReadFull(upstream, payload); err != nil || string(payload) != "payload" {
						t.Fatalf("tunnel: %v", err)
					}
				}
			}
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("shutdown: %v", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("proxy shutdown stuck")
			}
			assertClosed(t, client)
			if upstream != nil {
				assertClosed(t, upstream)
			}
		})
	}
}

func assertClosed(t *testing.T, conn net.Conn) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	var one [1]byte
	_, err := conn.Read(one[:])
	if err == nil {
		t.Fatal("connection still carries data after shutdown")
	}
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatal("connection was left open until timeout")
	}
}

func TestProxyTransportLogDropsRawPanicText(t *testing.T) {
	var output bytes.Buffer
	logger, err := logging.New(logging.Config{Writer: &output, Format: logging.FormatJSON})
	if err != nil {
		t.Fatal(err)
	}
	writer := proxyErrorWriter{ctx: logging.WithLogger(context.Background(), logger)}
	unsafe := []byte("http: panic serving guest: arbitrary sensitive data\nAuthorization: Bearer leaked\n")
	if n, err := writer.Write(unsafe); n != len(unsafe) || err != nil {
		t.Fatal(n, err)
	}
	if strings.Contains(output.String(), "sensitive") || strings.Contains(output.String(), "leaked") || strings.Contains(output.String(), "panic serving") {
		t.Fatal(output.String())
	}
	for _, expected := range []string{`"component":"proxy"`, `"operation":"serve_http"`, `"level":"ERROR"`, `"msg":"proxy HTTP transport failed"`} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("missing %s: %s", expected, output.String())
		}
	}
}

func TestTrackedConnectionPreservesTCPHalfClose(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	tracked := &proxyListener{Listener: listener, connections: make(map[*proxyConnection]struct{})}
	defer tracked.Close()
	client, err := net.Dial("tcp4", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server, err := tracked.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	_ = server.SetDeadline(time.Now().Add(time.Second))
	closeWrite(server)
	assertClosed(t, client)
	if _, err := client.Write([]byte("reply")); err != nil {
		t.Fatal(err)
	}
	var reply [5]byte
	if _, err := io.ReadFull(server, reply[:]); err != nil || string(reply[:]) != "reply" {
		t.Fatalf("half-close killed reverse traffic: %v", err)
	}
}
