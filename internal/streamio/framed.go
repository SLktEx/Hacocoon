package streamio

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"
)

const frameData byte = 0x10
const frameEOF byte = 0x11
const maxFrameData = 32 << 10

// FramedConn preserves half-close over a pair of process pipes. Closing a pipe
// aborts the transport; only the explicit EOF frame half-closes application data.
// One queued frame bounds unread data. No command, endpoint or authority is
// interpreted here. A deadline failure closes this transport; it is not retried.
type FramedConn struct {
	input                           io.ReadCloser
	output                          io.WriteCloser
	frames                          chan []byte
	done                            chan struct{}
	readMu                          sync.Mutex
	writeMu                         sync.Mutex
	buffer                          []byte
	eof                             bool
	writeEOF                        bool
	stateMu                         sync.Mutex
	cause                           error
	readTimer, writeTimer           *time.Timer
	readGeneration, writeGeneration uint64
	stopped                         chan struct{}
}

func NewFramedConn(input io.ReadCloser, output io.WriteCloser) *FramedConn {
	c := &FramedConn{input: input, output: output, frames: make(chan []byte, 1), done: make(chan struct{}), stopped: make(chan struct{})}
	go c.receive()
	return c
}
func (c *FramedConn) receive() {
	defer close(c.stopped)
	ended := false
	for {
		var header [5]byte
		if _, err := io.ReadFull(c.input, header[:]); err != nil {
			c.fail(io.ErrUnexpectedEOF)
			return
		}
		size := binary.BigEndian.Uint32(header[1:])
		if header[0] != frameData && header[0] != frameEOF || size > maxFrameData || header[0] == frameEOF && size != 0 || ended || header[0] == frameData && size == 0 {
			c.fail(errors.New("invalid byte transport frame"))
			return
		}
		var data []byte
		if header[0] == frameEOF {
			ended = true
		} else {
			data = make([]byte, int(size))
			if _, err := io.ReadFull(c.input, data); err != nil {
				c.fail(io.ErrUnexpectedEOF)
				return
			}
		}
		select {
		case c.frames <- data:
		case <-c.done:
			return
		}
	}
}
func (c *FramedConn) failure() error { c.stateMu.Lock(); defer c.stateMu.Unlock(); return c.cause }
func (c *FramedConn) fail(err error) {
	c.stateMu.Lock()
	changed := c.failLocked(err)
	c.stateMu.Unlock()
	if changed {
		c.closePipes()
	}
}
func (c *FramedConn) failLocked(err error) bool {
	if c.cause != nil {
		return false
	}
	c.cause = err
	if c.readTimer != nil {
		c.readTimer.Stop()
	}
	if c.writeTimer != nil {
		c.writeTimer.Stop()
	}
	close(c.done)
	return true
}
func (c *FramedConn) closePipes() { _ = c.input.Close(); _ = c.output.Close() }
func (c *FramedConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	c.readMu.Lock()
	defer c.readMu.Unlock()
	if c.eof {
		return 0, io.EOF
	}
	if len(c.buffer) == 0 {
		// Consume already received bytes before reporting a later pipe failure.
		select {
		case c.buffer = <-c.frames:
		default:
			select {
			case c.buffer = <-c.frames:
			case <-c.done:
				select {
				case c.buffer = <-c.frames:
				default:
					return 0, c.failure()
				}
			}
		}
		if c.buffer == nil {
			c.eof = true
			return 0, io.EOF
		}
	}
	n := copy(p, c.buffer)
	c.buffer = c.buffer[n:]
	return n, nil
}
func (c *FramedConn) writeFrame(kind byte, data []byte) error {
	var header [5]byte
	header[0] = kind
	binary.BigEndian.PutUint32(header[1:], uint32(len(data)))
	for _, part := range [][]byte{header[:], data} {
		for len(part) > 0 {
			n, err := c.output.Write(part)
			if n < 0 || n > len(part) {
				err = io.ErrShortWrite
				n = 0
			}
			part = part[n:]
			if err == nil && n == 0 {
				err = io.ErrShortWrite
			}
			if err != nil {
				c.fail(err)
				return err
			}
		}
	}
	return nil
}
func (c *FramedConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.writeEOF {
		return 0, io.ErrClosedPipe
	}
	select {
	case <-c.done:
		return 0, c.failure()
	default:
	}
	n := 0
	for len(p) > 0 {
		size := min(len(p), maxFrameData)
		if err := c.writeFrame(frameData, p[:size]); err != nil {
			return n, err
		}
		p = p[size:]
		n += size
	}
	return n, nil
}
func (c *FramedConn) CloseWrite() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if c.writeEOF {
		return nil
	}
	select {
	case <-c.done:
		return c.failure()
	default:
	}
	c.writeEOF = true
	return c.writeFrame(frameEOF, nil)
}
func (c *FramedConn) Close() error         { c.fail(net.ErrClosed); <-c.stopped; return nil }
func (c *FramedConn) LocalAddr() net.Addr  { return pipeAddress("local") }
func (c *FramedConn) RemoteAddr() net.Addr { return pipeAddress("peer") }

type pipeAddress string

func (pipeAddress) Network() string  { return "framed-pipe" }
func (a pipeAddress) String() string { return string(a) }
func (c *FramedConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}
func (c *FramedConn) setDeadline(t time.Time, read bool) error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	select {
	case <-c.done:
		return net.ErrClosed
	default:
	}
	timer, generation := &c.writeTimer, &c.writeGeneration
	if read {
		timer, generation = &c.readTimer, &c.readGeneration
	}
	*generation++
	expected := *generation
	if *timer != nil {
		(*timer).Stop()
	}
	if t.IsZero() {
		*timer = nil
		return nil
	}
	*timer = time.AfterFunc(time.Until(t), func() {
		c.stateMu.Lock()
		current := c.writeGeneration
		if read {
			current = c.readGeneration
		}
		expired := current == expected && c.failLocked(os.ErrDeadlineExceeded)
		c.stateMu.Unlock()
		if expired {
			c.closePipes()
		}
	})
	return nil
}
func (c *FramedConn) SetReadDeadline(t time.Time) error  { return c.setDeadline(t, true) }
func (c *FramedConn) SetWriteDeadline(t time.Time) error { return c.setDeadline(t, false) }
