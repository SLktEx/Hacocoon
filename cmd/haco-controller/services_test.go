package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestControllerServiceFailureStopsPeers(t *testing.T) {
	for _, failure := range []error{nil, context.Canceled, errors.New("listener failed")} {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		stopped := make(chan struct{})
		err := serveControllerServices(ctx,
			func(context.Context) error { return failure },
			func(ctx context.Context) error { <-ctx.Done(); close(stopped); return ctx.Err() },
		)
		if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("independent service exit was not a restartable failure: %v", err)
		}
		select {
		case <-stopped:
		default:
			t.Fatal("peer still running after return")
		}
	}
}

func TestControllerShutdownWaitsForBothServices(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 2)
	service := func(ctx context.Context) error { started <- struct{}{}; <-ctx.Done(); return ctx.Err() }
	done := make(chan error, 1)
	go func() { done <- serveControllerServices(ctx, service, service) }()
	<-started
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("controller did not stop")
	}
}

func TestControllerModeRequiresExactInstalledOption(t *testing.T) {
	if enabled, err := controllerMode([]string{"--standard-egress"}); !enabled || err != nil {
		t.Fatal(enabled, err)
	}
	if enabled, err := controllerMode(nil); enabled || err != nil {
		t.Fatal(enabled, err)
	}
	for _, args := range [][]string{{"--standard-egress=false"}, {"--listen", "0.0.0.0:18080"}, {"--standard-egress", "extra"}} {
		if _, err := controllerMode(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}
