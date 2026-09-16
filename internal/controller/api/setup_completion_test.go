package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/controller/transport"
	"github.com/SLktEx/Hacocoon/internal/host/recipes"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
)

// Simulate transport recognizing cancellation before ctx.Err observes its timer.
type setupReadErrorConn struct {
	net.Conn
	failure error
}

func (c setupReadErrorConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if errors.Is(err, io.EOF) {
		err = errors.Join(c.failure, err)
	}
	return n, err
}

func TestSetupProgressPreservesTransportCancellation(t *testing.T) {
	for _, failure := range []error{context.Canceled, context.DeadlineExceeded} {
		t.Run(failure.Error(), func(t *testing.T) {
			path := doctorTestSocket(t, func(s *control.Server) {
				_ = s.RegisterStream(MethodSetupProgress, func(context.Context, json.RawMessage) (control.Stream, error) {
					return func(_ context.Context, conn net.Conn) error {
						return json.NewEncoder(conn).Encode(setupCompletionFrame("setup", "running"))
					}, nil
				})
			})
			client, _ := NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
				conn, err := control.UnixDialer(path)(ctx)
				return setupReadErrorConn{Conn: conn, failure: failure}, err
			})
			if err := client.SetupHostProgress(context.Background(), recipes.Update{}, nil); !errors.Is(err, failure) {
				t.Fatalf("transport cancellation became protocol failure: %v", err)
			}
		})
	}
}

func TestSetupProgressWaitsForDelayedTerminal(t *testing.T) {
	for _, failed := range []bool{false, true} {
		name := "success"
		if failed {
			name = "failure"
		}
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{}, 1)
			defer close(release)
			path := doctorTestSocket(t, func(s *control.Server) {
				if err := RegisterSetup(s, setupServiceFunc(func(ctx context.Context) error {
					if err := hostsetup.Step(ctx, "client_provision", func() error { return nil }); err != nil {
						return err
					}
					// Further setup-owned work can finish after the last child event.
					select {
					case <-release:
					case <-ctx.Done():
						return ctx.Err()
					}
					if failed {
						return errors.New("SECRET-late-setup-failure")
					}
					return nil
				})); err != nil {
					t.Fatal(err)
				}
			})
			client, _ := NewClient(path)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			childSeen := make(chan struct{})
			var events []hostsetup.Event
			done := make(chan error, 1)
			go func() {
				done <- client.SetupHostProgress(ctx, recipes.Update{}, func(_ string, event hostsetup.Event) {
					events = append(events, event)
					if event.Stage == "client_provision" && event.State == "succeeded" {
						close(childSeen)
					}
				})
			}()
			select {
			case <-childSeen:
			case <-ctx.Done():
				t.Fatal("last child event was not observed")
			}
			select {
			case err := <-done:
				t.Fatalf("returned before terminal publication: %v", err)
			case <-time.After(500 * time.Millisecond):
			}
			// Release without closing so the deferred close also handles failures.
			release <- struct{}{}
			err := <-done
			wantState := "succeeded"
			if failed {
				wantState = "failed"
				var status *control.StatusError
				if !errors.As(err, &status) || status.Code != "setup_failed" {
					t.Fatalf("lost delayed failure: %v", err)
				}
			} else if err != nil {
				t.Fatalf("lost delayed success: %v", err)
			}
			if len(events) != 4 || events[0].Stage != "setup" || events[0].State != "running" || events[3].Stage != "setup" || events[3].State != wantState {
				t.Fatalf("expected one ordered setup terminal event: %+v", events)
			}
		})
	}
}

func TestSetupProgressUnregisteredChildKeepsControllerCompletion(t *testing.T) {
	for _, failure := range []error{nil, errors.New("SECRET-child-output")} {
		path := doctorTestSocket(t, func(s *control.Server) {
			if err := RegisterSetup(s, setupServiceFunc(func(ctx context.Context) error {
				// This was the unregistered stage in the failing #714 installer.
				return hostsetup.Step(ctx, "base_build_defaults", func() error { return failure })
			})); err != nil {
				t.Fatal(err)
			}
		})
		client, _ := NewClient(path)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var events []hostsetup.Event
		err := client.SetupHostProgress(ctx, recipes.Update{}, func(_ string, event hostsetup.Event) {
			events = append(events, event)
		})
		cancel()
		if failure == nil && err != nil {
			t.Fatal(err)
		}
		if failure != nil {
			var status *control.StatusError
			if !errors.As(err, &status) || status.Code != "setup_failed" || strings.Contains(err.Error(), "SECRET") {
				t.Fatal(err)
			}
		}
		if len(events) != 4 || events[1].Stage != "unknown" || events[2].Stage != "unknown" || events[3].Stage != "setup" {
			t.Fatalf("child replaced controller completion: %+v", events)
		}
	}
}

func setupCompletionFrame(stage, state string) setupFrame {
	return setupFrame{RequestID: strings.Repeat("a", 32), Event: &hostsetup.Event{Stage: stage, State: state}}
}

func TestSetupProgressRejectsTruncatedOrDuplicateCompletion(t *testing.T) {
	prefix := []setupFrame{
		setupCompletionFrame("setup", "running"),
		setupCompletionFrame("client_provision", "running"),
		setupCompletionFrame("client_provision", "succeeded"),
	}
	for name, suffix := range map[string][]setupFrame{
		"EOF before terminal":  nil,
		"ack without terminal": {{Done: true}},
		"EOF after terminal":   {setupCompletionFrame("setup", "succeeded")},
		"duplicate terminal":   {setupCompletionFrame("setup", "succeeded"), setupCompletionFrame("setup", "succeeded"), {Done: true}},
	} {
		t.Run(name, func(t *testing.T) {
			path := doctorTestSocket(t, func(s *control.Server) {
				_ = s.RegisterStream(MethodSetupProgress, func(context.Context, json.RawMessage) (control.Stream, error) {
					return func(_ context.Context, conn net.Conn) error {
						encoder := json.NewEncoder(conn)
						for _, frames := range [][]setupFrame{prefix, suffix} {
							for _, frame := range frames {
								if err := encoder.Encode(frame); err != nil {
									return err
								}
							}
						}
						return nil
					}, nil
				})
			})
			client, _ := NewClient(path)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := client.SetupHostProgress(ctx, recipes.Update{}, nil); !errors.Is(err, control.ErrProtocol) {
				t.Fatalf("accepted incomplete/duplicate completion: %v", err)
			}
		})
	}
}

func TestSetupProgressMissingTerminalWaitIsCancelable(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		name := "cancel"
		if deadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			release := make(chan struct{})
			defer close(release)
			path := doctorTestSocket(t, func(s *control.Server) {
				_ = RegisterSetup(s, setupServiceFunc(func(ctx context.Context) error {
					if err := hostsetup.Step(ctx, "client_provision", func() error { return nil }); err != nil {
						return err
					}
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				}))
			})
			client, _ := NewClient(path)
			ctx, cancel := context.WithCancel(context.Background())
			want := context.Canceled
			if deadline {
				cancel()
				ctx, cancel = context.WithTimeout(context.Background(), 500*time.Millisecond)
				want = context.DeadlineExceeded
			}
			defer cancel()
			childSeen := false
			started := time.Now()
			err := client.SetupHostProgress(ctx, recipes.Update{}, func(_ string, event hostsetup.Event) {
				if event.Stage == "client_provision" && event.State == "succeeded" {
					childSeen = true
					if !deadline {
						cancel()
					}
				}
			})
			if !childSeen || !errors.Is(err, want) || time.Since(started) > 3*time.Second {
				t.Fatalf("unbounded or incorrect cancellation: child=%v err=%v elapsed=%v", childSeen, err, time.Since(started))
			}
		})
	}
}
