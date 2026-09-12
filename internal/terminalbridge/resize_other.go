//go:build !linux

package terminalbridge

import "time"

// Native clients without Linux SIGWINCH sample console dimensions without
// consuming keyboard input. Only changed dimensions produce a control request.
func terminalSizeChanges() (<-chan time.Time, func()) {
	ticker := time.NewTicker(150 * time.Millisecond)
	return ticker.C, ticker.Stop
}
