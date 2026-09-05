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

const maxConcurrentConnections = 256

type Handler func(context.Context, json.RawMessage) (any, error)
type Stream func(context.Context, net.Conn) error
type StreamHandler func(context.Context, json.RawMessage) (Stream, error)

type Server struct {
	mu             sync.RWMutex
	handlers       map[string]Handler
	streamHandlers map[string]StreamHandler
	connections    chan struct{}

	sessionMu sync.Mutex
	sessions  map[string]*serverSession
}

func NewServer() *Server {
	return &Server{
		handlers:       make(map[string]Handler),
		streamHandlers: make(map[string]StreamHandler),
		connections:    make(chan struct{}, maxConcurrentConnections),
		sessions:       make(map[string]*serverSession),
	}
}

func (s *Server) Register(method string, handler Handler) error {
	if s == nil || strings.TrimSpace(method) == "" || handler == nil || strings.HasPrefix(method, "_control.") {
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
	if s == nil || strings.TrimSpace(method) == "" || handler == nil || strings.HasPrefix(method, "_control.") {
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
		select {
		case s.connections <- struct{}{}:
			go func() {
				defer func() { <-s.connections }()
				s.serveConn(ctx, conn)
			}()
		default:
			_ = conn.Close()
		}
	}
}

func (s *Server) serveConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	line, err := readEnvelopeLine(reader)
	if err != nil {
		if !errors.Is(err, io.EOF) {
			_ = writeJSONLine(conn, errorEnvelope(fmt.Errorf("read request: %w", err)))
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
	if request.Session && !request.Stream {
		_ = writeJSONLine(conn, errorEnvelope(&StatusError{Code: "invalid_argument", Message: "session requires stream mode"}))
		return
	}

	if request.Method == methodSessionWait {
		if request.Stream {
			_ = writeJSONLine(conn, errorEnvelope(&StatusError{Code: "invalid_argument", Message: "session wait is not a stream"}))
			return
		}
		var waitRequest sessionWaitRequest
		if err := json.Unmarshal(request.Payload, &waitRequest); err != nil || strings.TrimSpace(waitRequest.SessionID) == "" {
			_ = writeJSONLine(conn, errorEnvelope(&StatusError{Code: "invalid_argument", Message: "session_id is required"}))
			return
		}
		result, err := s.waitSession(ctx, waitRequest.SessionID)
		if err != nil {
			_ = writeJSONLine(conn, errorEnvelope(err))
			return
		}
		payload, err := marshalPayload(result)
		if err != nil {
			_ = writeJSONLine(conn, errorEnvelope(err))
			return
		}
		_ = writeJSONLine(conn, responseEnvelope{Version: ProtocolVersion, Payload: payload})
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

		response := responseEnvelope{Version: ProtocolVersion}
		var sessionID string
		var session *serverSession
		if request.Session {
			sessionID, session, err = s.createSession()
			if err != nil {
				_ = writeJSONLine(conn, errorEnvelope(err))
				return
			}
			response.SessionID = sessionID
		}
		if err := writeJSONLine(conn, response); err != nil {
			if session != nil {
				s.discardSession(sessionID, session)
			}
			return
		}

		streamErr := stream(ctx, &bufferedConn{Conn: conn, reader: reader})
		if session != nil {
			// Publish completion before the stream connection is closed by the
			// outer defer. A client that observes EOF can therefore immediately
			// fetch the result on the independent control connection.
			s.completeSession(sessionID, session, streamErr)
		}
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
	if len(payload)+1 > maxControlEnvelopeBytes {
		return fmt.Errorf("control envelope exceeds %d bytes: %w", maxControlEnvelopeBytes, ErrProtocol)
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
