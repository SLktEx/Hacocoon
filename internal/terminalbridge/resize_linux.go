//go:build linux

package terminalbridge

import (
	"os"
	"os/signal"
	"syscall"
)

func terminalSizeChanges() (<-chan os.Signal, func()) {
	changes := make(chan os.Signal, 1)
	signal.Notify(changes, syscall.SIGWINCH)
	return changes, func() { signal.Stop(changes) }
}
