package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
)

type setupServiceFunc func(context.Context) error

func (f setupServiceFunc) SetupHost(ctx context.Context) error { return f(ctx) }

func TestSetupUsesBoundedServiceAndRejectsCallerParameters(t *testing.T) {
	var calls atomic.Int32
	path := doctorTestSocket(t, func(s *control.Server) {
		if err := RegisterSetup(s, setupServiceFunc(func(ctx context.Context) error {
			calls.Add(1)
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > setupTimeout {
				t.Error("missing server deadline")
			}
			return nil
		})); err != nil {
			t.Fatal(err)
		}
	})
	client, _ := NewClient(path)
	if err := client.SetupHost(context.Background()); err != nil {
		t.Fatal(err)
	}
	wire, _ := control.NewClient(control.UnixDialer(path))
	for _, request := range []any{map[string]string{"source": "/tmp/attacker"}, map[string]bool{"force": true}, []string{"haco"}, "repair"} {
		err := wire.Call(context.Background(), MethodSetup, request, nil)
		var status *control.StatusError
		if !errors.As(err, &status) || status.Code != "invalid_argument" {
			t.Fatalf("parameters accepted: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("service calls=%d", calls.Load())
	}
}

func TestSetupRejectsConcurrentCallsAndAllowsExplicitRetry(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	path := doctorTestSocket(t, func(s *control.Server) {
		_ = RegisterSetup(s, setupServiceFunc(func(context.Context) error {
			if calls.Add(1) == 1 {
				close(entered)
				<-release
			}
			return nil
		}))
	})
	client, _ := NewClient(path)
	done := make(chan error, 1)
	go func() { done <- client.SetupHost(context.Background()) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("setup did not start")
	}
	err := client.SetupHost(context.Background())
	close(release)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "busy" {
		t.Fatalf("concurrent setup error=%v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := client.SetupHost(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("calls=%d", calls.Load())
	}
}

func TestSetupFailureIsSanitizedAndDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	path := doctorTestSocket(t, func(s *control.Server) {
		_ = RegisterSetup(s, setupServiceFunc(func(context.Context) error {
			calls.Add(1)
			return errors.New("arbitrary-backend-secret")
		}))
	})
	client, _ := NewClient(path)
	err := client.SetupHost(context.Background())
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "setup_failed" || strings.Contains(err.Error(), "arbitrary-backend-secret") {
		t.Fatalf("unsafe failure=%v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("automatic retry=%d", calls.Load())
	}
}

func TestSetupRejectsMissingOrWrongAcknowledgement(t *testing.T) {
	for _, response := range []any{nil, PingResponse{}, PingResponse{ProtocolVersion: 999}} {
		path := doctorTestSocket(t, func(s *control.Server) {
			_ = s.Register(MethodSetup, func(context.Context, json.RawMessage) (any, error) { return response, nil })
		})
		client, _ := NewClient(path)
		if err := client.SetupHost(context.Background()); !errors.Is(err, control.ErrProtocol) {
			t.Fatalf("accepted response=%v err=%v", response, err)
		}
	}
}

func TestSetupLostClientDoesNotAllowOverlappingMutation(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	path := doctorTestSocket(t, func(s *control.Server) {
		_ = RegisterSetup(s, setupServiceFunc(func(context.Context) error { close(entered); <-release; return nil }))
	})
	client, _ := NewClient(path)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.SetupHost(ctx) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		cancel()
		close(release)
		t.Fatal("setup did not start")
	}
	cancel()
	if err := <-done; err == nil {
		close(release)
		t.Fatal("canceled client succeeded")
	}
	err := client.SetupHost(context.Background())
	close(release)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "busy" {
		t.Fatalf("lost connection allowed overlapping setup: %v", err)
	}
}
