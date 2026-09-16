//go:build !linux

package run

import (
	"context"
	"os"
	"os/signal"
)

func withTerminationSignals(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, os.Interrupt)
}
