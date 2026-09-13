package control

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

// Process framing is negotiated by a distinct method, never inferred from bytes
// on a legacy raw stream. Input EOF is a frame; socket EOF means disconnection.
const (
	processInput byte = iota + 1
	processInputEOF
	processOutput
	processErrorOutput
	processCredit
	processResult
	processInputStop
	processChunk        = 32 << 10
	MaxProcessResult    = 64 << 10
	processWriteTimeout = 30 * time.Second
)

func readProcessFrame(r io.Reader) (byte, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	size := binary.BigEndian.Uint32(header[1:])
	limit := uint32(processChunk)
	if header[0] == processResult {
		limit = MaxProcessResult
	}
	if size > limit {
		return 0, nil, ErrProtocol
	}
	payload := make([]byte, int(size))
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return header[0], payload, nil
}

type processWriter struct {
	mu           sync.Mutex
	conn         net.Conn
	fail         func(error)
	finished     bool
	inputStopped bool
}

func (w *processWriter) frame(kind byte, payload []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.finished {
		return io.ErrClosedPipe
	}
	if w.inputStopped && kind != processResult {
		return io.ErrClosedPipe
	}
	if kind == processInputStop {
		w.inputStopped = true
	}
	if kind == processResult {
		w.finished = true
	}
	var header [5]byte
	header[0] = kind
	binary.BigEndian.PutUint32(header[1:], uint32(len(payload)))
	err := w.conn.SetWriteDeadline(time.Now().Add(processWriteTimeout))
	if err == nil {
		err = writeProcessBytes(w.conn, header[:])
	}
	if err == nil {
		err = writeProcessBytes(w.conn, payload)
	}
	if err != nil && w.fail != nil {
		w.fail(err)
	}
	return err
}

func writeProcessBytes(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if n < 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

type processOutputWriter struct {
	frames *processWriter
	kind   byte
}

func (w processOutputWriter) Write(data []byte) (int, error) {
	written := 0
	for len(data) > 0 {
		n := min(len(data), processChunk)
		if err := w.frames.frame(w.kind, data[:n]); err != nil {
			return written, err
		}
		written += n
		data = data[n:]
	}
	return written, nil
}
