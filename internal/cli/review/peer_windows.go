package reviewcli

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/client/review"
	"github.com/SLktEx/Hacocoon/internal/platform/winprocess"
	"github.com/SLktEx/Hacocoon/internal/platform/wsl/coord"
)

// The only channel carrying controller tokens or answers is these anonymous
// child pipes. No inherited terminal, user environment or public endpoint.
type processReviewPeer struct {
	peer    *desktopreview.Peer
	process *winprocess.Private
	stop    func() bool
	mu      sync.Mutex
}

func startReviewPeer(ctx context.Context, plan desktopreview.Invocation, own string) (*processReviewPeer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	process, err := winprocess.Start(plan.File, plan.Args, plan.Env, filepath.Dir(own))
	if err != nil {
		return nil, errors.New("start private review")
	}
	p := &processReviewPeer{peer: desktopreview.NewPeer(process.Output, process.Input), process: process}
	p.stop = context.AfterFunc(ctx, func() { _ = process.Close() })
	return p, nil
}
func (p *processReviewPeer) Close() error {
	p.stop()
	return p.process.Close()
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
	stop := context.AfterFunc(ctx, func() { _ = p.Close() })
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
			cleanupErr := peer.Close()
			peer = nil
			if cleanupErr != nil {
				// Keep the launch reservation until this owner exits. An
				// unconfirmed descendant must not race storage reclamation.
				err = errors.Join(err, errors.New("private review cleanup not confirmed"))
				return
			}
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
