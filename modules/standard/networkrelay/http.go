package networkrelay

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const upgrade = "hacocoon-network-v1"

type Handler struct {
	Service *Service
	Sources SourceResolver
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil || h.Service == nil || h.Sources == nil {
		http.Error(w, "network unavailable", http.StatusServiceUnavailable)
		return
	}
	if r.Method != http.MethodPost || r.RequestURI != Path || r.URL == nil || r.URL.IsAbs() || r.ContentLength < 1 || r.ContentLength > 4096 || r.Header.Get("Upgrade") != upgrade || r.Header.Get("Connection") != "Upgrade" {
		http.Error(w, "invalid connection request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
		http.Error(w, "unmanaged source", http.StatusForbidden)
		return
	}
	environment, instance, err := h.Sources.ResolveEnvironmentInstance(r.Context(), ip)
	if err != nil {
		http.Error(w, "unmanaged source", http.StatusForbidden)
		return
	}
	var spec Spec
	decoder := json.NewDecoder(io.LimitReader(r.Body, 4097))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&spec) != nil {
		http.Error(w, "invalid connection request", http.StatusBadRequest)
		return
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF {
		http.Error(w, "invalid connection request", http.StatusBadRequest)
		return
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "connection transport unavailable", http.StatusServiceUnavailable)
		return
	}
	connection, err := h.Service.Open(r.Context(), Source{Environment: environment, Instance: instance}, spec)
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, core.ErrPolicyDenied) || errors.Is(err, core.ErrApprovalDenied) {
			status = http.StatusForbidden
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"reason": failureReason(err)})
		return
	}
	defer connection.Close()
	client, buffer, err := hijacker.Hijack()
	if err != nil {
		connection.Fail("transport_failed")
		return
	}
	defer client.Close()
	if buffer.Reader.Buffered() != 0 {
		connection.Fail("invalid_request")
		return
	}
	if connection.BindClient(client) != nil {
		return
	}
	view, _ := json.Marshal(connection.Session)
	if _, err = buffer.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: " + upgrade + "\r\nX-Haco-Network-Session: " + base64.RawURLEncoding.EncodeToString(view) + "\r\n\r\n"); err != nil {
		return
	}
	if buffer.Flush() != nil {
		return
	}
	if spec.Protocol == "udp" {
		relayDatagrams(connection, client)
		return
	}
	if err := relayTCP(connection.Conn, client); err != nil {
		connection.Fail(failureReason(err))
	}
}

// Each direction finishes independently: EOF on a request stream must still
// allow the destination to send its complete response.
func relayTCP(upstream, client net.Conn) error {
	done := make(chan error, 2)
	go func() { _, err := io.Copy(upstream, client); halfClose(upstream); done <- err }()
	go func() { _, err := io.Copy(client, upstream); halfClose(client); done <- err }()
	first := <-done
	if first != nil {
		_ = client.Close()
		_ = upstream.Close()
	}
	return errors.Join(first, <-done)
}
func halfClose(conn net.Conn) {
	if closer, ok := conn.(interface{ CloseWrite() error }); ok {
		_ = closer.CloseWrite()
	}
}
func readDatagram(reader io.Reader) ([]byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(reader, header[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(header[:]))
	if n > MaxDatagram {
		return nil, core.ErrInvalidArgument
	}
	data := make([]byte, n)
	_, err := io.ReadFull(reader, data)
	return data, err
}
func writeDatagram(writer io.Writer, data []byte) error {
	if len(data) > MaxDatagram {
		return core.ErrInvalidArgument
	}
	frame := make([]byte, 2+len(data))
	binary.BigEndian.PutUint16(frame[:2], uint16(len(data)))
	copy(frame[2:], data)
	for len(frame) > 0 {
		n, err := writer.Write(frame)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}
func relayDatagrams(connection *Connection, client net.Conn) {
	relayDatagramsWithIdle(connection, client, 30*time.Second)
}
func relayDatagramsWithIdle(connection *Connection, client net.Conn, idle time.Duration) {
	done := make(chan string, 2)
	activity := make(chan struct{}, 1)
	touch := func() {
		select {
		case activity <- struct{}{}:
		default:
		}
	}
	go func() {
		for {
			data, err := readDatagram(client)
			if err != nil {
				done <- failureReason(err)
				return
			}
			if err := connection.Validate(); err != nil {
				done <- failureReason(err)
				return
			}
			if _, err = connection.Conn.Write(data); err != nil {
				done <- failureReason(err)
				return
			}
			touch()
		}
	}()
	go func() {
		buffer := make([]byte, MaxDatagram+1)
		for {
			n, err := connection.Conn.Read(buffer)
			if err != nil {
				done <- failureReason(err)
				return
			}
			if n > MaxDatagram {
				done <- "invalid_datagram"
				return
			}
			if err := connection.Validate(); err != nil {
				done <- failureReason(err)
				return
			}
			if err := writeDatagram(client, buffer[:n]); err != nil {
				done <- failureReason(err)
				return
			}
			touch()
		}
	}()
	timer := time.NewTimer(idle)
	defer timer.Stop()
	for {
		select {
		case reason := <-done:
			connection.service.finish(connection.active, reason, false)
			<-done
			return
		case <-timer.C:
			connection.service.finish(connection.active, "udp_idle", false)
			<-done
			<-done
			return
		case <-activity:
			timer.Reset(idle)
		}
	}
}
