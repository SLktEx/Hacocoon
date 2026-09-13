package client

import (
	"github.com/SLktEx/Hacocoon/internal/core"
	"net"
	"sync"
)

// Transport ownership is process-local; durable authority stays in the catalog
// and provider. Register and revoke run under the same lifecycle transition lock.
type streamBinding struct{ environment, instance, grant string }
type ownedStream struct {
	net.Conn
	owner   *Service
	binding streamBinding
	once    sync.Once
	err     error
}

func (s *Service) trackStream(binding streamBinding, conn net.Conn) net.Conn {
	c := &ownedStream{Conn: conn, owner: s, binding: binding}
	s.streamMu.Lock()
	defer s.streamMu.Unlock()
	if s.streams == nil {
		s.streams = make(map[streamBinding]map[*ownedStream]struct{})
	}
	if s.streams[binding] == nil {
		s.streams[binding] = make(map[*ownedStream]struct{})
	}
	s.streams[binding][c] = struct{}{}
	return c
}
func (s *Service) closeStreams(binding streamBinding) {
	s.streamMu.Lock()
	var connections []*ownedStream
	for c := range s.streams[binding] {
		connections = append(connections, c)
	}
	s.streamMu.Unlock()
	for _, c := range connections {
		_ = c.Close()
	}
}
func (c *ownedStream) CloseWrite() error {
	if half, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return half.CloseWrite()
	}
	return core.ErrUnsupported
}
func (c *ownedStream) Close() error {
	c.once.Do(func() {
		c.err = c.Conn.Close()
		c.owner.streamMu.Lock()
		defer c.owner.streamMu.Unlock()
		delete(c.owner.streams[c.binding], c)
		if len(c.owner.streams[c.binding]) == 0 {
			delete(c.owner.streams, c.binding)
		}
	})
	return c.err
}
