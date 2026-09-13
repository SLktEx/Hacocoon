package streamio

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"sync"
)

// StartFramedProcess owns anonymous pipes and reaps exactly its own child.
// The caller selects the trusted executable, fixed arguments and minimal env.
// Explicit os.Pipe ownership keeps cmd.Wait from truncating unread stdout.
func StartFramedProcess(ctx context.Context, cmd *exec.Cmd) (net.Conn, error) {
	if cmd == nil || cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
		return nil, errors.New("unconfigured private process required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		inputRead.Close()
		inputWrite.Close()
		return nil, err
	}
	closePipes := func() { inputRead.Close(); inputWrite.Close(); outputRead.Close(); outputWrite.Close() }
	// A file avoids exec's stderr copy goroutine, which could otherwise keep
	// Wait blocked if a descendant retained the inherited stderr pipe.
	discard, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		closePipes()
		return nil, err
	}
	defer discard.Close()
	cmd.Stdin = inputRead
	cmd.Stdout = outputWrite
	cmd.Stderr = discard
	if err = cmd.Start(); err != nil {
		closePipes()
		return nil, err
	}
	inputRead.Close()
	outputWrite.Close()
	p := &processConn{FramedConn: NewFramedConn(outputRead, inputWrite), cmd: cmd, done: make(chan struct{}), closed: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(p.done) }()
	go func() {
		select {
		case <-ctx.Done():
			_ = p.Close()
		case <-p.closed:
		}
	}()
	if ctx.Err() != nil {
		p.Close()
		return nil, ctx.Err()
	}
	return p, nil
}

type processConn struct {
	*FramedConn
	cmd    *exec.Cmd
	done   chan struct{}
	once   sync.Once
	closed chan struct{}
}

func (p *processConn) Close() error {
	p.once.Do(func() {
		close(p.closed)
		_ = p.FramedConn.Close()
		select {
		case <-p.done:
		default:
			_ = p.cmd.Process.Kill()
			<-p.done
		}
	})
	return nil
}
