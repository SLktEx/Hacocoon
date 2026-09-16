package reviewcli

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/SLktEx/Hacocoon/internal/client/review"
	"github.com/SLktEx/Hacocoon/internal/platform/wsl/coord"
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

// Ready never selects or answers a request. It shares the startup deadline and
// existing cancellation/reaping with normal exchanges; no failed read is retried.
func (p *processReviewPeer) Ready(ctx context.Context) error {
	reply, err := p.Exchange(ctx, desktopreview.Message{Action: "list"})
	if err != nil {
		return err
	}
	if reply.Type != "pending" || reply.Error != "" {
		return errors.New("private review unavailable")
	}
	return nil
}
func (p *processReviewPeer) Exchange(ctx context.Context, m desktopreview.Message) (desktopreview.Reply, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if ctx.Err() != nil {
		return desktopreview.Reply{}, ctx.Err()
	}
	stop := context.AfterFunc(ctx, p.Close)
	defer stop()
	reply, err := p.peer.Exchange(m)
	if ctx.Err() != nil {
		return desktopreview.Reply{}, ctx.Err()
	}
	return reply, err
}

// The gate is held until a read-only round trip completes. Releasing it after
// Start alone permits a delayed WSL child to restart a distribution after stop.
func startReadyReviewPeer(lifetime, startup context.Context, plan desktopreview.Invocation, own, distribution string) (peer *processReviewPeer, err error) {
	startup, cancelStartup := context.WithTimeout(startup, desktopreview.StartupTimeout)
	defer cancelStartup()
	guard, err := wslcoord.AcquireLaunch(distribution)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil && peer != nil {
			peer.Close()
			peer = nil
		}
		err = errors.Join(err, guard.Close())
		if err != nil && peer != nil {
			peer.Close()
			peer = nil
		}
	}()
	peer, err = startReviewPeer(lifetime, plan, own)
	if err == nil {
		err = peer.Ready(startup)
	}
	return
}
