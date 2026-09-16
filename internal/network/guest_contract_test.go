package networkrelay

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// A controller response is untrusted at the guest boundary. Exercise its wire
// representation and verify that refusal also closes the underlying socket.
func guestResponseServer(t *testing.T, response string) (string, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer func() { _ = conn.Close() }()
		if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
			done <- err
			return
		}
		request, err := http.ReadRequest(bufio.NewReader(conn))
		if err != nil {
			done <- err
			return
		}
		_, readErr := io.Copy(io.Discard, request.Body)
		if err := errors.Join(readErr, request.Body.Close()); err != nil {
			done <- err
			return
		}
		if request.Method != http.MethodPost || request.RequestURI != Path || request.Header.Get("Upgrade") != upgrade {
			done <- errors.New("guest sent an unexpected connection request")
			return
		}
		// The guest may refuse an oversized response before the write completes.
		_, _ = io.WriteString(conn, response)
		var b [1]byte
		n, err := conn.Read(b[:])
		if n != 0 || err == nil {
			done <- errors.New("refused guest socket remained usable")
			return
		}
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			done <- errors.New("guest did not close refused socket")
			return
		}
		done <- nil
	}()
	return listener.Addr().String(), done
}

func TestGuestRefusesMalformedOrMismatchedUpgradeAndClosesSocket(t *testing.T) {
	spec := Spec{Kind: "external", Target: "example.invalid", Protocol: "tcp", Port: 443, DurationSeconds: 10}
	for _, mode := range []string{"invalid-http", "oversized-header", "invalid-base64", "oversized-session", "invalid-json", "upgrade", "id", "state", "name", "kind", "protocol", "port", "expired", "extended-lifetime"} {
		t.Run(mode, func(t *testing.T) {
			view := Session{ID: "connection", State: "active", Target: Target{Kind: spec.Kind, Name: spec.Target, Protocol: spec.Protocol, Port: spec.Port}, ExpiresAt: time.Now().Add(5 * time.Second)}
			switch mode {
			case "id":
				view.ID = ""
			case "state":
				view.State = "pending"
			case "name":
				view.Target.Name = "other.invalid"
			case "kind":
				view.Target.Kind = "host"
			case "protocol":
				view.Target.Protocol = "udp"
			case "port":
				view.Target.Port = 8443
			case "expired":
				view.ExpiresAt = time.Now().Add(-time.Second)
			case "extended-lifetime":
				view.ExpiresAt = time.Now().Add(time.Hour)
			}
			data, err := json.Marshal(view)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "invalid-json" {
				data = []byte("{")
			}
			if mode == "oversized-session" {
				data = []byte(strings.Repeat(" ", 8193))
			}
			encoded := base64.RawURLEncoding.EncodeToString(data)
			if mode == "invalid-base64" {
				encoded = "!invalid!"
			}
			protocol := upgrade
			if mode == "upgrade" {
				protocol = "unrelated"
			}
			response := "HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: " + protocol + "\r\nX-Haco-Network-Session: " + encoded + "\r\n\r\n"
			if mode == "invalid-http" {
				response = "invalid\r\n\r\n"
			}
			if mode == "oversized-header" {
				response = "HTTP/1.1 101 Switching Protocols\r\nX-Padding: " + strings.Repeat("a", 16384) + "\r\n\r\n"
			}
			endpoint, done := guestResponseServer(t, response)
			conn, session, err := openGuestAt(context.Background(), endpoint, spec)
			if conn != nil || session.ID != "" || !errors.Is(err, core.ErrIncompatibleState) {
				t.Fatal("untrusted response published a connection", session, err)
			}
			if err := waitClientResult(t, done); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestGuestFailureExposesOnlyBoundedKnownReasons(t *testing.T) {
	spec := Spec{Kind: "external", Target: "example.invalid", Protocol: "tcp", Port: 443, DurationSeconds: 10}
	for _, fixture := range []struct{ name, body, want string }{
		{"known", `{"reason":"identity_changed"}`, "network connection: identity_changed"},
		{"unknown", `{"reason":"credential=secret-token"}`, "network connection rejected (HTTP 502)"},
		{"malformed", `{"reason":`, "network connection rejected (HTTP 502)"},
		{"oversized", strings.Repeat("private backend output", 300), "network connection rejected (HTTP 502)"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			response := fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\nContent-Length: %d\r\n\r\n%s", len(fixture.body), fixture.body)
			endpoint, done := guestResponseServer(t, response)
			conn, session, err := openGuestAt(context.Background(), endpoint, spec)
			if conn != nil || session.ID != "" || err == nil || err.Error() != fixture.want {
				t.Fatal("failure response leaked untrusted details or produced authority", session, err)
			}
			if err := waitClientResult(t, done); err != nil {
				t.Fatal(err)
			}
		})
	}
}
