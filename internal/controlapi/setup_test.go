package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

type setupServiceFunc func(context.Context) error

func (f setupServiceFunc) SetupHost(ctx context.Context, _ recipes.Update) error { return f(ctx) }

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
	if err := client.SetupHost(context.Background(), recipes.Update{}); err != nil {
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
	go func() { done <- client.SetupHost(context.Background(), recipes.Update{}) }()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("setup did not start")
	}
	err := client.SetupHost(context.Background(), recipes.Update{})
	close(release)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "busy" {
		t.Fatalf("concurrent setup error=%v", err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := client.SetupHost(context.Background(), recipes.Update{}); err != nil {
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
	err := client.SetupHost(context.Background(), recipes.Update{})
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
		if err := client.SetupHost(context.Background(), recipes.Update{}); !errors.Is(err, control.ErrProtocol) {
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
	go func() { done <- client.SetupHost(ctx, recipes.Update{}) }()
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
	err := client.SetupHost(context.Background(), recipes.Update{})
	close(release)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "busy" {
		t.Fatalf("lost connection allowed overlapping setup: %v", err)
	}
}

type setupUpdateFunc func(context.Context, recipes.Update) error

func (f setupUpdateFunc) SetupHost(ctx context.Context, u recipes.Update) error { return f(ctx, u) }
func TestSetupRecipeIntentAndSanitizedFailure(t *testing.T) {
	script := "echo synthetic-setup-secret\n"
	old := logging.Root()
	var logs bytes.Buffer
	logging.SetRoot(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { logging.SetRoot(old) })
	path := doctorTestSocket(t, func(s *control.Server) {
		_ = RegisterSetup(s, setupUpdateFunc(func(ctx context.Context, u recipes.Update) error {
			if u.Script == nil || *u.Script != script || u.Clear {
				t.Error("recipe intent lost")
			}
			return errors.Join(recipes.ErrExecutionFailed, errors.New(script))
		}))
	})
	client, _ := NewClient(path)
	err := client.SetupHost(context.Background(), recipes.Update{Script: &script})
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "customization_failed" {
		t.Fatalf("status=%v", err)
	}
	if strings.Contains(err.Error(), "synthetic-setup-secret") || strings.Contains(logs.String(), "synthetic-setup-secret") {
		t.Fatal("recipe leaked into diagnostics")
	}
	if !strings.Contains(logs.String(), "Trusted Host customization failed") {
		t.Fatal("missing owning failure event")
	}
}
