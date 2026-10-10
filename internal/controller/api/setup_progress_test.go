package controlapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/host/recipes"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
	"github.com/SLktEx/Hacocoon/internal/logging"
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

func TestNotificationSetupFailureReachesProgressAndJournalOnce(t *testing.T) {
	old := logging.Root()
	var logs bytes.Buffer
	logging.SetRoot(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer logging.SetRoot(old)
	calls := 0
	path := doctorTestSocket(t, func(s *control.Server) {
		_ = RegisterSetup(s, setupServiceFunc(func(ctx context.Context) error {
			calls++
			return hostsetup.Step(ctx, "notification_setup", func() error {
				return errors.Join(&hostsetup.NotificationServiceFailure{Operation: "restart"}, errors.New("SECRET-backend-output"))
			})
		}))
	})
	c, _ := NewClient(path)
	var events []hostsetup.Event
	err := c.SetupHostProgress(context.Background(), recipes.Update{}, func(_ string, e hostsetup.Event) { events = append(events, e) })
	var status *control.StatusError
	if !errors.As(err, &status) || status.Code != "setup_failed" || strings.Contains(err.Error(), "SECRET") {
		t.Fatal("lost sanitized failure", err)
	}
	seen := false
	for _, event := range events {
		if event.Stage == "notification_setup" && event.State == "failed" && event.Reason == "notification_restart_failed" {
			seen = true
		}
	}
	if !seen || calls != 1 || strings.Contains(logs.String(), "SECRET") || !strings.Contains(logs.String(), "notification_restart_failed") || strings.Count(logs.String(), `"level":"ERROR"`) != 1 {
		t.Fatal("failure was lost, repeated or exposed raw output", calls, logs.String())
	}
}

func TestDefaultImageStagesReachProgressAndOwnOneFailureLog(t *testing.T) {
	stages := []string{"default_image_read", "default_image_project", "default_image_resolve", "default_image_copy", "default_image_write"}
	for failed := -1; failed < len(stages); failed++ {
		name := "success"
		if failed >= 0 {
			name = stages[failed]
		}
		t.Run(name, func(t *testing.T) {
			old := logging.Root()
			var logs bytes.Buffer
			logging.SetRoot(slog.New(slog.NewJSONHandler(&logs, nil)))
			defer logging.SetRoot(old)
			calls := 0
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterSetup(s, setupServiceFunc(func(ctx context.Context) error {
					for n, stage := range stages {
						err := func() (err error) {
							defer hostsetup.Track(ctx, stage)(&err)
							calls++
							if n == failed {
								return errors.New("SECRET /private/path https://private.invalid?token=SECRET")
							}
							return nil
						}()
						if err != nil {
							return err
						}
					}
					return nil
				})); err != nil {
					t.Fatal(err)
				}
			})
			client, err := NewClient(path)
			if err != nil {
				t.Fatal(err)
			}
			var events []hostsetup.Event
			err = client.SetupHostProgress(context.Background(), recipes.Update{}, func(id string, e hostsetup.Event) {
				if len(id) != 32 {
					t.Error("invalid correlation ID")
				}
				events = append(events, e)
			})
			wantCalls, wantErrors, terminal := len(stages), 0, "succeeded"
			if failed >= 0 {
				wantCalls, wantErrors, terminal = failed+1, 1, "failed"
				var status *control.StatusError
				if !errors.As(err, &status) || status.Code != "setup_failed" {
					t.Fatal("invalid failure frame", err)
				}
			} else if err != nil {
				t.Fatal("invalid success frame", err)
			}
			if calls != wantCalls || len(events) != 2*wantCalls+2 || events[len(events)-1].Stage != "setup" || events[len(events)-1].State != terminal {
				t.Fatal(calls, events)
			}
			for n := 0; n < wantCalls; n++ {
				start, end := events[2*n+1], events[2*n+2]
				state := "succeeded"
				if n == failed {
					state = "failed"
				}
				if start.Stage != stages[n] || start.State != "running" || end.Stage != stages[n] || end.State != state {
					t.Fatal(events)
				}
			}
			if strings.Count(logs.String(), `"level":"ERROR"`) != wantErrors || strings.Contains(logs.String(), "SECRET") || strings.Contains(logs.String(), "/private/") || strings.Contains(logs.String(), "https://") {
				t.Fatal("failure logging ownership or redaction changed", logs.String())
			}
		})
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
