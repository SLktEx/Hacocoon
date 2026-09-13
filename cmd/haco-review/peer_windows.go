package main

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/desktopreview"
)

// The only channel carrying controller tokens or answers is these anonymous
// child pipes. No inherited terminal, user environment or public endpoint.
type processReviewPeer struct {
	peer      *desktopreview.Peer
	cancel    context.CancelFunc
	input     io.WriteCloser
	output    io.ReadCloser
	done      chan error
	closeOnce sync.Once
	mu        sync.Mutex
}

func startReviewPeer(ctx context.Context, plan desktopreview.Invocation, own string) (*processReviewPeer, error) {
	ctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(ctx, plan.File, plan.Args...)
	cmd.Env = plan.Env
	cmd.Dir = filepath.Dir(own)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.WaitDelay = 2 * time.Second
	// Raw subprocess diagnostics may contain credentials; discard them.
	cmd.Stderr = io.Discard
	input, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		input.Close()
		cancel()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		input.Close()
		output.Close()
		cancel()
		return nil, errors.New("start private review")
	}
	p := &processReviewPeer{peer: desktopreview.NewPeer(output, input), cancel: cancel, input: input, output: output, done: make(chan error, 1)}
	go func() { p.done <- cmd.Wait() }()
	return p, nil
}
func (p *processReviewPeer) Close() {
	p.closeOnce.Do(func() { p.cancel(); p.input.Close(); p.output.Close(); <-p.done })
}
func (p *processReviewPeer) Exchange(ctx context.Context, m desktopreview.Message) (desktopreview.Reply, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil {
		return desktopreview.Reply{}, ctx.Err()
	}
	stop := context.AfterFunc(ctx, p.Close)
	defer stop()
	return p.peer.Exchange(m)
}
