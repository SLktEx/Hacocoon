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
	connections := serveConnections{active: make(map[*servedConnection]struct{})}
	stopWatch, watchDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-ctx.Done():
			_ = listener.Close()
		case <-stopWatch:
		}
	}()
	defer func() {
		close(stopWatch)
		connections.close()
		<-watchDone
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
			owned := connections.add(conn)
			go func() {
				defer func() {
					connections.remove(owned)
					<-s.connections
				}()
				s.serveConn(ctx, conn)
			}()
		default:
			_ = conn.Close()
		}
	}
}

// Each Serve owns only its accepted transports. Closing them interrupts I/O;
// arbitrary handlers may still be running and keep their shared connection slot.
type serveConnections struct {
	mu     sync.Mutex
	active map[*servedConnection]struct{}
}

// A separate identity avoids requiring a comparable net.Conn implementation or
// wrapping away optional connection methods such as CloseWrite.
type servedConnection struct{ conn net.Conn }

func (c *serveConnections) add(conn net.Conn) *servedConnection {
	owned := &servedConnection{conn: conn}
	c.mu.Lock()
	c.active[owned] = struct{}{}
	c.mu.Unlock()
	return owned
}

func (c *serveConnections) remove(owned *servedConnection) {
	c.mu.Lock()
	delete(c.active, owned)
	c.mu.Unlock()
}

func (c *serveConnections) close() {
	c.mu.Lock()
	active := make([]net.Conn, 0, len(c.active))
	for owned := range c.active {
		active = append(active, owned.conn)
	}
	c.mu.Unlock()
	for _, conn := range active {
		_ = conn.Close()
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

	if request.Method == methodSessionResize {
		if request.Stream {
			_ = writeJSONLine(conn, errorEnvelope(NewStatusError("invalid_argument", "terminal resize is not a stream")))
			return
		}
		if err := s.resizeSession(request.Payload); err != nil {
			_ = writeJSONLine(conn, errorEnvelope(err))
		} else {
			_ = writeJSONLine(conn, responseEnvelope{Version: ProtocolVersion})
		}
		return
	}

	if request.Method == methodSessionCancel {
		var cancelRequest sessionWaitRequest
		if request.Stream || json.Unmarshal(request.Payload, &cancelRequest) != nil {
			_ = writeJSONLine(conn, errorEnvelope(ErrInvalidArgument))
			return
		}
		if err := s.cancelSession(ctx, cancelRequest.SessionID); err != nil {
			_ = writeJSONLine(conn, errorEnvelope(err))
		} else {
			_ = writeJSONLine(conn, responseEnvelope{Version: ProtocolVersion})
		}
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
		terminal := &terminalControl{}
		if request.Session {
			ctx = context.WithValue(ctx, terminalControlKey{}, terminal)
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
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			defer cancel()
			sessionID, session, err = s.createSession(terminal, cancel)
			if err != nil {
				_ = writeJSONLine(conn, errorEnvelope(err))
				return
			}
			response.SessionID = sessionID
			response.TerminalResize = terminal.resize != nil
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
