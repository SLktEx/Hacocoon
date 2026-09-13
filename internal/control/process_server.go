package control

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"time"
)

// ServeProcess carries bounded process streams and one opaque final receipt.
// The operation owns lifecycle and cleanup and must return only after cleanup.
// Credit bounds queued input while leaving the wire reader free to observe a
// disconnect even when the process never consumes stdin. No input-stall timeout
// or unbounded producer queue is needed.
func ServeProcess(ctx context.Context, conn net.Conn, operation func(context.Context, io.Reader, io.Writer, io.Writer) ([]byte, error)) error {
	if ctx == nil || conn == nil || operation == nil {
		return ErrInvalidArgument
	}
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	frames := &processWriter{conn: conn, fail: cancel}
	input := &processInputReader{ctx: ctx, frames: frames, wake: make(chan struct{}, 1), drained: make(chan struct{}), credit: processChunk}
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			kind, data, err := readProcessFrame(conn)
			if err != nil {
				cancel(err)
				return
			}
			if err = input.accept(kind, data); err != nil {
				cancel(err)
				return
			}
		}
	}()
	defer func() { _ = conn.Close(); <-stopped }()
	watchDone := make(chan struct{})
	defer close(watchDone)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-watchDone:
		}
	}()
	var credit [4]byte
	binary.BigEndian.PutUint32(credit[:], processChunk)
	if err := frames.frame(processCredit, credit[:]); err != nil {
		return err
	}
	result, err := operation(ctx, input, processOutputWriter{frames, processOutput}, processOutputWriter{frames, processErrorOutput})
	input.finish()
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	if len(result) == 0 || len(result) > MaxProcessResult {
		return ErrProtocol
	}
	// Stop producers and drain their final EOF before closing a Unix socket.
	// Closing with unread input may reset the peer and discard the final result.
	if err := conn.SetReadDeadline(time.Now().Add(processWriteTimeout)); err != nil {
		return err
	}
	if err := frames.frame(processInputStop, nil); err != nil {
		return err
	}
	select {
	case <-input.drained:
	case <-ctx.Done():
		return context.Cause(ctx)
	}
	return frames.frame(processResult, result)
}

type processInputReader struct {
	mu       sync.Mutex
	ctx      context.Context
	frames   *processWriter
	wake     chan struct{}
	data     []byte
	credit   int
	eof      bool
	finished bool
	drained  chan struct{}
}

func (r *processInputReader) finish() {
	r.mu.Lock()
	r.finished = true
	r.mu.Unlock()
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *processInputReader) accept(kind byte, data []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.eof {
		return ErrProtocol
	}
	switch kind {
	case processInput:
		if len(data) == 0 || len(data) > r.credit {
			return ErrProtocol
		}
		r.credit -= len(data)
		r.data = append(r.data, data...)
	case processInputEOF:
		if len(data) != 0 {
			return ErrProtocol
		}
		r.eof = true
		close(r.drained)
	default:
		return ErrProtocol
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
	return nil
}

func (r *processInputReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		if r.ctx.Err() != nil {
			return 0, context.Cause(r.ctx)
		}
		r.mu.Lock()
		if r.finished {
			r.mu.Unlock()
			return 0, io.EOF
		}
		if len(r.data) > 0 {
			n := copy(p, r.data)
			r.data = r.data[n:]
			r.credit += n
			r.mu.Unlock()
			var credit [4]byte
			binary.BigEndian.PutUint32(credit[:], uint32(n))
			if err := r.frames.frame(processCredit, credit[:]); err != nil {
				return n, err
			}
			return n, nil
		}
		eof := r.eof
		r.mu.Unlock()
		if eof {
			return 0, io.EOF
		}
		select {
		case <-r.ctx.Done():
			return 0, context.Cause(r.ctx)
		case <-r.wake:
		}
	}
}
