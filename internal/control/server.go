package control

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

var noDeadline time.Time

type Handler func(context.Context, json.RawMessage) (any, error)
type Stream func(context.Context, net.Conn) error
type StreamHandler func(context.Context, json.RawMessage) (Stream, error)

type Server struct {
	mu             sync.RWMutex
	handlers       map[string]Handler
	streamHandlers map[string]StreamHandler
}

func NewServer() *Server {
	return &Server{
		handlers:       make(map[string]Handler),
		streamHandlers: make(map[string]StreamHandler),
	}
}

func (s *Server) Register(method string, handler Handler) error {
	if s == nil || strings.TrimSpace(method) == "" || handler == nil {
		return ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.handlers[method]; ok {
		return fmt.Errorf("control method %q: %w", method, ErrInvalidArgument)
	}
	if _, ok := s.streamHandlers[method]; ok {
		return fmt.Errorf("control method %q: %w", method, ErrInvalidArgument)
	}
	s.handlers[method] = handler
	return nil
}

func (s *Server) RegisterStream(method string, handler StreamHandler) error {
	if s == nil || strings.TrimSpace(method) == "" || handler == nil {
		return ErrInvalidArgument
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.handlers[method]; ok {
		return fmt.Errorf("control method %q: %w", method, ErrInvalidArgument)
	}
	if _, ok := s.streamHandlers[method]; ok {
		return fmt.Errorf("control method %q: %w", method, ErrInvalidArgument)
	}
	s.streamHandlers[method] = handler
	return nil
}

func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	if s == nil || listener == nil {
		return ErrInvalidArgument
	}
	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return ctx.Err()
			}
			return fmt.Errorf("accept control connection: %w", err)
		}
		go s.serveConn(ctx, conn)
	}
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			_ = writeJSONLine(conn, errorEnvelope(fmt.Errorf("read request: %w", ErrProtocol)))
		}
		return
	}
	var request requestEnvelope
	if err := json.Unmarshal(line, &request); err != nil {
		_ = writeJSONLine(conn, errorEnvelope(fmt.Errorf("decode request: %w", ErrProtocol)))
		return
	}
	if request.Version != ProtocolVersion {
		_ = writeJSONLine(conn, errorEnvelope(&StatusError{Code: "protocol_version", Message: fmt.Sprintf("unsupported version %d", request.Version)}))
		return
	}
	if strings.TrimSpace(request.Method) == "" {
		_ = writeJSONLine(conn, errorEnvelope(&StatusError{Code: "invalid_argument", Message: "method is required"}))
		return
	}

	if request.Stream {
		s.mu.RLock()
		handler := s.streamHandlers[request.Method]
		s.mu.RUnlock()
		if handler == nil {
			_ = writeJSONLine(conn, errorEnvelope(&StatusError{Code: "not_found", Message: "stream method not found"}))
			return
		}
		stream, err := handler(ctx, request.Payload)
		if err != nil {
			_ = writeJSONLine(conn, errorEnvelope(err))
			return
		}
		if stream == nil {
			_ = writeJSONLine(conn, errorEnvelope(&StatusError{Code: "internal", Message: "stream handler returned no stream"}))
			return
		}
		if err := writeJSONLine(conn, responseEnvelope{Version: ProtocolVersion}); err != nil {
			return
		}
		_ = stream(ctx, &bufferedConn{Conn: conn, reader: reader})
		return
	}

	s.mu.RLock()
	handler := s.handlers[request.Method]
	s.mu.RUnlock()
	if handler == nil {
		_ = writeJSONLine(conn, errorEnvelope(&StatusError{Code: "not_found", Message: "method not found"}))
		return
	}
	value, err := handler(ctx, request.Payload)
	if err != nil {
		_ = writeJSONLine(conn, errorEnvelope(err))
		return
	}
	payload, err := marshalPayload(value)
	if err != nil {
		_ = writeJSONLine(conn, errorEnvelope(err))
		return
	}
	_ = writeJSONLine(conn, responseEnvelope{Version: ProtocolVersion, Payload: payload})
}

func writeJSONLine(writer io.Writer, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = writer.Write(payload)
	return err
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}
