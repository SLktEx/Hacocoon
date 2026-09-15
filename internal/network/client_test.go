package networkrelay

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func waitClientResult(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("network worker did not terminate")
		return nil
	}
}

func TestLocalTCPClientsReceiveIndependentAuthorityAndPreserveHalfClose(t *testing.T) {
	listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	spec := Spec{Kind: "external", Target: "example.invalid", Protocol: "tcp", Port: 443, DurationSeconds: 10}
	upstreams := make(chan net.Conn, 2)
	responses := make(chan error, 2)
	for i := 0; i < 2; i++ {
		upstream, server := tcpPair(t)
		upstreams <- upstream
		go func() {
			data, err := io.ReadAll(server)
			if err == nil {
				_, err = server.Write(append([]byte("response:"), data...))
			}
			responses <- errors.Join(err, server.CloseWrite())
		}()
	}
	observed := make(chan Session, 2)
	var requests atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- ServeTCP(ctx, listener, spec, func(_ context.Context, got Spec) (net.Conn, Session, error) {
			if got != spec {
				return nil, Session{}, core.ErrInvalidArgument
			}
			id := requests.Add(1)
			return <-upstreams, Session{ID: fmt.Sprintf("grant-%d", id), ExpiresAt: time.Now().Add(3 * time.Second)}, nil
		}, func(s Session, err error) {
			if err == nil {
				observed <- s
			}
		})
	}()
	for i := 0; i < 2; i++ {
		client, err := net.DialTCP("tcp", nil, listener.Addr().(*net.TCPAddr))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		payload := []byte{byte(i), 0, 255, 10}
		if _, err := client.Write(payload); err != nil {
			t.Fatal(err)
		}
		if err := client.CloseWrite(); err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(client)
		if err != nil || !bytes.Equal(got, append([]byte("response:"), payload...)) {
			t.Fatal("half-close lost response", got, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := waitClientResult(t, responses); err != nil {
			t.Fatal(err)
		}
	}
	first, second := <-observed, <-observed
	if requests.Load() != 2 || first.ID == second.ID || first.ID == "" || second.ID == "" {
		t.Fatal("listener authority was reused", requests.Load(), first, second)
	}
	cancel()
	if err := waitClientResult(t, done); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestLocalTCPBoundsPendingApprovalsAndClosesRejectedClient(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan struct{}, 17)
	released := make(chan error, 16)
	done := make(chan error, 1)
	go func() {
		done <- ServeTCP(ctx, listener, Spec{}, func(ctx context.Context, _ Spec) (net.Conn, Session, error) {
			entered <- struct{}{}
			<-ctx.Done()
			released <- ctx.Err()
			return nil, Session{}, ctx.Err()
		}, nil)
	}()
	for i := 0; i < 16; i++ {
		conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		select {
		case <-entered:
		case <-time.After(3 * time.Second):
			t.Fatal("approval request did not start")
		}
	}
	overflow, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = overflow.Close() }()
	if err := overflow.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if n, err := overflow.Read(b[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("overflow client was retained", n, err)
	}
	select {
	case <-entered:
		t.Fatal("seventeenth client requested authority")
	default:
	}
	cancel()
	if err := waitClientResult(t, done); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for i := 0; i < 16; i++ {
		if err := waitClientResult(t, released); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
}

func TestLocalTCPReportsApprovalDenialAndAcceptFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := make(chan error, 1)
	done := make(chan error, 1)
	go func() {
		done <- ServeTCP(ctx, listener, Spec{}, func(context.Context, Spec) (net.Conn, Session, error) { return nil, Session{}, core.ErrPolicyDenied }, func(_ Session, err error) { observed <- err })
	}()
	client, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	if err := client.SetReadDeadline(time.Now().Add(3 * time.Second)); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if n, err := client.Read(b[:]); n != 0 || !errors.Is(err, io.EOF) {
		t.Fatal("denied client received data or remained open", n, err)
	}
	if err := waitClientResult(t, observed); !errors.Is(err, core.ErrPolicyDenied) {
		t.Fatal("approval refusal lost", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := waitClientResult(t, done); !errors.Is(err, net.ErrClosed) {
		t.Fatal("accept failure lost", err)
	}
}

func TestLocalUDPAssociationsKeepDatagramsAndPeersSeparate(t *testing.T) {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	spec := Spec{Kind: "external", Target: "example.invalid", Protocol: "udp", Port: 53, DurationSeconds: 10}
	var requests atomic.Int32
	workers := make(chan error, 2)
	done := make(chan error, 1)
	go func() {
		done <- ServeUDP(ctx, listener, spec, func(_ context.Context, got Spec) (net.Conn, Session, error) {
			if got != spec {
				return nil, Session{}, core.ErrInvalidArgument
			}
			requests.Add(1)
			client, server := net.Pipe()
			go func() {
				defer func() { _ = server.Close() }()
				for {
					data, err := readDatagram(server)
					if err != nil {
						workers <- err
						return
					}
					if err := writeDatagram(server, append([]byte("reply:"), data...)); err != nil {
						workers <- err
						return
					}
				}
			}()
			return client, Session{ID: "association", ExpiresAt: time.Now().Add(3 * time.Second)}, nil
		}, nil)
	}()
	for peer := 0; peer < 2; peer++ {
		client, err := net.DialUDP("udp", nil, listener.LocalAddr().(*net.UDPAddr))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = client.Close() })
		if err := client.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			t.Fatal(err)
		}
		for packet := 0; packet < 2; packet++ {
			data := []byte{byte(peer), byte(packet), 0, 255}
			if _, err := client.Write(data); err != nil {
				t.Fatal(err)
			}
			var reply [64]byte
			n, err := client.Read(reply[:])
			if err != nil || !bytes.Equal(reply[:n], append([]byte("reply:"), data...)) {
				t.Fatal("datagram/peer identity lost", reply[:n], err)
			}
		}
	}
	if requests.Load() != 2 {
		t.Fatal("UDP peer did not retain its own association", requests.Load())
	}
	cancel()
	if err := waitClientResult(t, done); err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, net.ErrClosed) {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := waitClientResult(t, workers); !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrClosedPipe) {
			t.Fatal("association transport was not closed", err)
		}
	}
}

func TestRelayListenersRejectWildcardAndNonNumericLoopback(t *testing.T) {
	for _, address := range []string{"0.0.0.0:1234", "[::]:1234", "localhost:1234", "[fe80::1%lo]:1", "invalid", "192.0.2.1:80"} {
		if err := validateLoopback(address); !errors.Is(err, core.ErrInvalidArgument) {
			t.Fatal(address, err)
		}
	}
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	if err := ServeTCP(context.Background(), listener, Spec{}, nil, nil); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal("wildcard TCP listener accepted", err)
	}
	udp, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = udp.Close() }()
	if err := ServeUDP(context.Background(), udp, Spec{}, nil, nil); !errors.Is(err, core.ErrInvalidArgument) {
		t.Fatal("wildcard UDP listener accepted", err)
	}
	left, right := net.Pipe()
	defer func() { _ = left.Close(); _ = right.Close() }()
	if err := (&guestConn{Conn: left}).CloseWrite(); !errors.Is(err, core.ErrUnsupported) {
		t.Fatal("unsupported half-close hidden", err)
	}
}
