package reclaimnative

import (
	"context"
	"errors"
	"time"
)

// No termination capability is present. read must be nonblocking, bounded to
// len(dst), and distinguish temporary no-data from all writers being closed.
// exit observes only the exact held kernel process, not a reopened PID.
type observedProcess interface {
	read([]byte) (int, bool, error)
	exit() (bool, uint32, error)
	close() error
}

func observe(ctx context.Context, p observedProcess) (raw []byte, err error) {
	defer func() { err = errors.Join(err, p.close()) }()
	raw = make([]byte, 0, receiptLimit)
	var buffer [receiptLimit + 1]byte
	eof := false
	for {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if !eof {
			var n int
			n, eof, err = p.read(buffer[:])
			if err != nil {
				return nil, err
			}
			if n < 0 || n > len(buffer) || n > receiptLimit-len(raw) {
				return nil, errors.New("helper output exceeds receipt bound")
			}
			raw = append(raw, buffer[:n]...)
		}
		done, code, readErr := p.exit()
		if readErr != nil {
			return nil, readErr
		}
		if done && code != 0 {
			return nil, errors.New("helper failed; operation outcome unconfirmed")
		}
		if eof && done {
			return raw, nil
		}
		// EOF without confirmed successful exit is uncertainty, not nondispatch.
		// Root exit without EOF can retain an inherited writer: keep only the bound.
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
