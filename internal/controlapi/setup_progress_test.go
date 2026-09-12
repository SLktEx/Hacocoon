package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/recipes"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"
)

func TestSetupProgressBeforeCompletionAndJournal(t *testing.T) {
	old := logging.Root()
	var logs bytes.Buffer
	logging.SetRoot(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer logging.SetRoot(old)
	entered, release := make(chan struct{}), make(chan struct{})
	path := doctorTestSocket(t, func(s *control.Server) {
		_ = RegisterSetup(s, setupServiceFunc(func(ctx context.Context) error {
			return hostsetup.Step(ctx, "wsl_interop", func() error { close(entered); <-release; return errors.New("SECRET-guest-output") })
		}))
	})
	c, _ := NewClient(path)
	seen := make(chan hostsetup.Event, 20)
	done := make(chan error, 1)
	go func() {
		done <- c.SetupHostProgress(context.Background(), recipes.Update{}, func(id string, e hostsetup.Event) {
			if len(id) != 32 {
				t.Error(id)
			}
			seen <- e
		})
	}()
	<-entered
	select {
	case e := <-seen:
		if e.State != "running" {
			t.Error(e)
		}
	case <-time.After(time.Second):
		t.Error("no live progress")
	}
	// Both transport paths share the same lifecycle exclusion.
	err := c.SetupHost(context.Background(), recipes.Update{})
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "busy" {
		t.Error(err)
	}
	close(release)
	if err := <-done; !errors.As(err, &status) || status.Code != "setup_failed" {
		t.Fatal(err)
	}
	close(seen)
	failed := false
	for e := range seen {
		if e.Stage == "wsl_interop" && e.State == "failed" && e.Reason == "failed" {
			failed = true
		}
	}
	if !failed || strings.Contains(logs.String(), "SECRET") || !strings.Contains(logs.String(), "request_id") || strings.Count(logs.String(), `"level":"ERROR"`) != 1 {
		t.Fatal(logs.String())
	}
}
func TestSetupProgressRejectsUntrustedFramesAndMissingCompletion(t *testing.T) {
	for _, frame := range []any{
		setupFrame{Done: true},
		setupFrame{Event: &hostsetup.Event{Stage: "SECRET", State: "running"}, RequestID: strings.Repeat("a", 32)},
		setupFrame{Event: &hostsetup.Event{Stage: "setup", State: "failed", Reason: "SECRET"}, RequestID: strings.Repeat("a", 32)},
		setupFrame{Event: &hostsetup.Event{Stage: "setup", State: "running"}, RequestID: "SECRET"},
		setupFrame{Event: &hostsetup.Event{Stage: "setup", State: "running"}, RequestID: strings.Repeat("a", 32)},
	} {
		path := doctorTestSocket(t, func(s *control.Server) {
			_ = s.RegisterStream(MethodSetupProgress, func(context.Context, json.RawMessage) (control.Stream, error) {
				return func(_ context.Context, c net.Conn) error { return json.NewEncoder(c).Encode(frame) }, nil
			})
		})
		c, _ := NewClient(path)
		if err := c.SetupHostProgress(context.Background(), recipes.Update{}, nil); !errors.Is(err, control.ErrProtocol) {
			t.Fatal(frame, err)
		}
	}
}
func TestSetupProgressDisconnectKeepsExclusion(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	path := doctorTestSocket(t, func(s *control.Server) {
		_ = RegisterSetup(s, setupServiceFunc(func(context.Context) error { close(entered); <-release; return nil }))
	})
	c, _ := NewClient(path)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- c.SetupHostProgress(ctx, recipes.Update{}, nil) }()
	<-entered
	cancel()
	if err := <-done; err == nil {
		t.Error("canceled observation succeeded")
	}
	err := c.SetupHost(context.Background(), recipes.Update{})
	close(release)
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "busy" {
		t.Fatal(err)
	}
}
