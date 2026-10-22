//go:build linux

package cli

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Progress is user output on stderr, not raw backend output or an application
// log. Join the ticker before returning so callers can safely reuse the writer.
func openStage(ctx context.Context, out io.Writer, stage string, run func() error) error {
	label := cliMessage("open.stage." + stage)
	if _, err := fmt.Fprintln(out, cliMessage("open.progress", label)); err != nil {
		return err
	}
	done, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, _ = fmt.Fprintln(out, cliMessage("open.waiting", label))
			}
		}
	}()
	err := run()
	close(done)
	<-joined
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	_, err = fmt.Fprintln(out, cliMessage("open.finished", label))
	return err
}
