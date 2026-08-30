package control

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	maxProxyProtocolLine = 108
	proxyHeaderTimeout   = 5 * time.Second
)

// ProxyProtocolListener accepts HAProxy PROXY protocol v1 connections only
// from loopback. It is used behind an Incus bind=instance proxy device so the
// controller can recover the originating Environment IP without exposing a
// routable Host listener to the sandbox network.
type ProxyProtocolListener struct {
	net.Listener
}

func NewProxyProtocolListener(listener net.Listener) (*ProxyProtocolListener, error) {
	if listener == nil {
		return nil, ErrInvalidArgument
	}
	return &ProxyProtocolListener{Listener: listener}, nil
}

func (l *ProxyProtocolListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	wrapped, err := acceptProxyProtocolConn(conn)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return wrapped, nil
}

func acceptProxyProtocolConn(conn net.Conn) (net.Conn, error) {
	if conn == nil {
		return nil, ErrInvalidArgument
	}
	peer, ok := conn.RemoteAddr().(*net.TCPAddr)
	if !ok || peer.IP == nil || !peer.IP.IsLoopback() {
		return nil, fmt.Errorf("PROXY protocol transport must originate on loopback: %w", ErrProtocol)
	}
	if err := conn.SetReadDeadline(time.Now().Add(proxyHeaderTimeout)); err != nil {
		return nil, err
	}
	reader := bufio.NewReaderSize(conn, maxProxyProtocolLine+1)
	line, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read PROXY protocol header: %w", ErrProtocol)
	}
	if len(line) > maxProxyProtocolLine || !strings.HasSuffix(line, "\r\n") {
		return nil, fmt.Errorf("invalid PROXY protocol header length: %w", ErrProtocol)
	}
	fields := strings.Fields(strings.TrimSuffix(line, "\r\n"))
	if len(fields) != 6 || fields[0] != "PROXY" || (fields[1] != "TCP4" && fields[1] != "TCP6") {
		return nil, fmt.Errorf("unsupported PROXY protocol header: %w", ErrProtocol)
	}
	source := net.ParseIP(fields[2])
	destination := net.ParseIP(fields[3])
	if source == nil || destination == nil || source.IsUnspecified() || source.IsLoopback() || source.IsMulticast() {
		return nil, fmt.Errorf("invalid PROXY source address: %w", ErrProtocol)
	}
	if fields[1] == "TCP4" && source.To4() == nil {
		return nil, fmt.Errorf("PROXY address family mismatch: %w", ErrProtocol)
	}
	if fields[1] == "TCP6" && source.To4() != nil {
		return nil, fmt.Errorf("PROXY address family mismatch: %w", ErrProtocol)
	}
	sourcePort, err := strconv.Atoi(fields[4])
	if err != nil || sourcePort < 1 || sourcePort > 65535 {
		return nil, fmt.Errorf("invalid PROXY source port: %w", ErrProtocol)
	}
	destinationPort, err := strconv.Atoi(fields[5])
	if err != nil || destinationPort < 1 || destinationPort > 65535 {
		return nil, fmt.Errorf("invalid PROXY destination port: %w", ErrProtocol)
	}
	if err := conn.SetReadDeadline(noDeadline); err != nil {
		return nil, err
	}
	return &proxyProtocolConn{
		Conn:   conn,
		reader: reader,
		remote: &net.TCPAddr{IP: append(net.IP(nil), source...), Port: sourcePort},
		local:  &net.TCPAddr{IP: append(net.IP(nil), destination...), Port: destinationPort},
	}, nil
}

type proxyProtocolConn struct {
	net.Conn
	reader *bufio.Reader
	remote net.Addr
	local  net.Addr
}

func (c *proxyProtocolConn) Read(p []byte) (int, error) {
	if c == nil || c.reader == nil {
		return 0, errors.New("invalid PROXY protocol connection")
	}
	return c.reader.Read(p)
}

func (c *proxyProtocolConn) RemoteAddr() net.Addr { return c.remote }
func (c *proxyProtocolConn) LocalAddr() net.Addr  { return c.local }
