//go:build windows && (amd64 || arm64)

package reclaimnative

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"time"

	"github.com/SLktEx/Hacocoon/internal/cli/ui"
	"github.com/SLktEx/Hacocoon/internal/platform/wsl/reclaim"
)

func Main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, cliui.Resolve(os.Getenv), actions{
		resolve: wslreclaim.ResolveNativeRegistration,
		latest: func(ctx context.Context, registration string) (savedStatus, error) {
			status, err := wslreclaim.ReadLatestPreparedStatus(ctx, registration)
			if err != nil {
				return savedStatus{}, err
			}
			raw, err := json.Marshal(status)
			return savedStatus{operation: status.Operation, state: status.State, document: raw}, err
		},
		open: openInstalledHelper, confirm: confirmConsole,
	})
	cancel()
	stop()
	os.Exit(code)
}
