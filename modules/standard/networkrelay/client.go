package networkrelay

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const GuestAddress = "169.254.254.1:18080"

// OpenGuest carries only a data connection request. Source identity is assigned
// by the controller from the guarded ingress, never by this guest process.
func OpenGuest(ctx context.Context, spec Spec) (net.Conn, Session, error) {
	return openGuestAt(ctx, GuestAddress, spec)
}
func openGuestAt(ctx context.Context, endpoint string, spec Spec) (net.Conn, Session, error) {
	if err := validateSpec(spec); err != nil {
		return nil, Session{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(spec.DurationSeconds)*time.Second)
	conn, err := (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, "tcp", endpoint)
	if err != nil {
		cancel()
		return nil, Session{}, err
	}
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	ok := false
	defer func() {
		if !ok {
			stop()
			cancel()
			conn.Close()
		}
	}()
	deadline, _ := ctx.Deadline()
	_ = conn.SetDeadline(deadline)
	data, _ := json.Marshal(spec)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+endpoint+Path, bytes.NewReader(data))
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", upgrade)
	if err = req.Write(conn); err != nil {
		return nil, Session{}, err
	}
	// Bound the complete HTTP header without buffering any relay payload.
	header := make([]byte, 0, 1024)
	var b [1]byte
	for !bytes.HasSuffix(header, []byte("\r\n\r\n")) {
		if len(header) >= 16384 {
			return nil, Session{}, core.ErrIncompatibleState
		}
		if _, err = io.ReadFull(conn, b[:]); err != nil {
			return nil, Session{}, err
		}
		header = append(header, b[0])
	}
	response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(header)), req)
	if err != nil {
		return nil, Session{}, core.ErrIncompatibleState
	}
	if response.StatusCode != http.StatusSwitchingProtocols {
		var failure struct {
			Reason string `json:"reason"`
		}
		if response.ContentLength > 0 && response.ContentLength <= 4096 {
			body := make([]byte, response.ContentLength)
			if _, err := io.ReadFull(conn, body); err == nil {
				_ = json.Unmarshal(body, &failure)
			}
		}
		switch failure.Reason {
		case "policy_denied", "approval_denied", "identity_changed", "dns_failed", "connection_refused", "unreachable", "timeout", "policy_expired_or_changed", "limit_reached", "target_not_found":
			return nil, Session{}, fmt.Errorf("network connection: %s", failure.Reason)
		default:
			return nil, Session{}, fmt.Errorf("network connection rejected (HTTP %d)", response.StatusCode)
		}
	}
	encoded := response.Header.Get("X-Haco-Network-Session")
	view, err := base64.RawURLEncoding.DecodeString(encoded)
	var session Session
	if err != nil || len(view) > 8192 || json.Unmarshal(view, &session) != nil ||
		response.Header.Get("Upgrade") != upgrade || session.ID == "" || session.State != "active" ||
		session.Target.Name != spec.Target || session.Target.Kind != spec.Kind || session.Target.Protocol != spec.Protocol ||
		(spec.Port != 0 && session.Target.Port != spec.Port) || !session.ExpiresAt.After(time.Now()) || session.ExpiresAt.After(deadline.Add(time.Second)) {
		return nil, Session{}, core.ErrIncompatibleState
	}
	ok = true
	return &guestConn{Conn: conn, stop: stop, cancel: cancel}, session, nil
}

type guestConn struct {
	net.Conn
	stop   func() bool
	cancel context.CancelFunc
}

func (c *guestConn) Close() error { c.stop(); c.cancel(); return c.Conn.Close() }
func (c *guestConn) CloseWrite() error {
	if writer, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return writer.CloseWrite()
	}
	return core.ErrUnsupported
}

type GuestDial func(context.Context, Spec) (net.Conn, Session, error)

// ServeTCP accepts ordinary local TCP clients. Each accepted connection needs
// its own authorization; listening itself does not create a reusable grant.
func ServeTCP(ctx context.Context, listener net.Listener, spec Spec, dial GuestDial, observe func(Session, error)) error {
	if err := validateLoopback(listener.Addr().String()); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { listener.Close() })
	defer stop()
	slots := make(chan struct{}, 16)
	for {
		client, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return err
		}
		select {
		case slots <- struct{}{}:
		default:
			client.Close()
			continue
		}
		go func() {
			defer func() { <-slots }()
			defer client.Close()
			conn, view, err := dial(ctx, spec)
			if observe != nil {
				observe(view, err)
			}
			if err != nil {
				return
			}
			defer conn.Close()
			stop := context.AfterFunc(ctx, func() { client.Close(); conn.Close() })
			defer stop()
			_ = client.SetDeadline(view.ExpiresAt)
			if err := relayTCP(conn, client); err != nil && observe != nil {
				observe(view, err)
			}
		}()
	}
}
func validateLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() || strings.Contains(host, "%") {
		return core.ErrInvalidArgument
	}
	return nil
}

// ServeUDP gives each local peer one connected destination and response path.
// There is no guest-selected destination in the framing. Queues and associations
// are bounded while a human approval is pending.
func ServeUDP(ctx context.Context, listener *net.UDPConn, spec Spec, dial GuestDial, observe func(Session, error)) error {
	if err := validateLoopback(listener.LocalAddr().String()); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { listener.Close() })
	defer stop()
	type incoming struct {
		peer *net.UDPAddr
		data []byte
		err  error
	}
	packets := make(chan incoming, 16)
	go func() {
		defer close(packets)
		for {
			buffer := make([]byte, MaxDatagram+1)
			n, peer, err := listener.ReadFromUDP(buffer)
			packet := incoming{peer, buffer[:n], err}
			select {
			case packets <- packet:
			case <-ctx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()
	closed := make(chan string, 16)
	peers := map[string]chan []byte{}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case key := <-closed:
			delete(peers, key)
		case packet, ok := <-packets:
			if !ok {
				return io.EOF
			}
			if packet.err != nil {
				return packet.err
			}
			if len(packet.data) > MaxDatagram || !packet.peer.IP.IsLoopback() {
				continue
			}
			key := packet.peer.String()
			queue := peers[key]
			if queue == nil {
				if len(peers) >= 16 {
					continue
				}
				queue = make(chan []byte, 8)
				peers[key] = queue
				go func(peer *net.UDPAddr, key string, queue <-chan []byte) {
					defer func() {
						select {
						case closed <- key:
						case <-ctx.Done():
						}
					}()
					conn, view, err := dial(ctx, spec)
					if observe != nil {
						observe(view, err)
					}
					if err != nil {
						return
					}
					defer conn.Close()
					peerCtx, cancel := context.WithCancel(ctx)
					defer cancel()
					stop := context.AfterFunc(peerCtx, func() { conn.Close() })
					defer stop()
					done := make(chan error, 1)
					go func() {
						for {
							data, err := readDatagram(conn)
							if err != nil {
								done <- err
								return
							}
							if _, err = listener.WriteToUDP(data, peer); err != nil {
								done <- err
								return
							}
						}
					}()
					for {
						select {
						case <-ctx.Done():
							return
						case err := <-done:
							if err != nil && !errors.Is(err, io.EOF) && observe != nil {
								observe(view, err)
							}
							return
						case data := <-queue:
							if err := writeDatagram(conn, data); err != nil {
								return
							}
						}
					}
				}(packet.peer, key, queue)
			}
			select {
			case queue <- packet.data:
			default:
			}
		}
	}
}
