package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/controlapi"
	"github.com/SLktEx/Hacocoon/internal/desktopreview"
)

func TestPrivateReviewWaitsForControllerBeforeReadingRequests(t *testing.T) {
	for _, name := range []string{"late controller", "rejected ping", "failed request"} {
		t.Run(name, func(t *testing.T) {
			server := control.NewServer()
			var pings, requests atomic.Int32
			if err := server.Register(controlapi.MethodPing, func(context.Context, json.RawMessage) (any, error) {
				pings.Add(1)
				if name == "rejected ping" {
					return nil, control.NewStatusError("refused", "controller refused")
				}
				return controlapi.PingResponse{ProtocolVersion: 1}, nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := server.Register(controlapi.MethodApprovalPending, func(context.Context, json.RawMessage) (any, error) {
				requests.Add(1)
				if name == "failed request" {
					return nil, control.NewStatusError("unavailable", "request failed")
				}
				return []controlapi.ApprovalRequestPayload{}, nil
			}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "control.sock")
			listener, err := control.ListenUnix(path, 0600)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- server.Serve(ctx, listener) }()
			defer func() { cancel(); <-done }()
			var calls atomic.Int32
			client, err := controlapi.NewClientWithDialer(func(ctx context.Context) (net.Conn, error) {
				if calls.Add(1) == 1 {
					return nil, control.ErrUnavailable
				}
				return control.UnixDialer(path)(ctx)
			})
			if err != nil {
				t.Fatal(err)
			}
			input := bytes.NewBufferString("{\"version\":1,\"sequence\":1,\"action\":\"list\"}\n")
			var output bytes.Buffer
			err = serveDesktopReview(ctx, client, input, &output)
			if name == "rejected ping" {
				var refused *control.StatusError
				if !errors.As(err, &refused) || pings.Load() != 1 || requests.Load() != 0 || input.Len() == 0 || output.Len() != 0 {
					t.Fatal("rejected readiness consumed or replayed input")
				}
				return
			}
			if err != nil || pings.Load() != 1 || requests.Load() != 1 || calls.Load() != 3 {
				t.Fatal("startup or request repeated", err, calls.Load())
			}
			var reply desktopreview.Reply
			if json.Unmarshal(output.Bytes(), &reply) != nil {
				t.Fatal("invalid private reply")
			}
			if name == "failed request" {
				if reply.Type != "error" || reply.Error != "pending_unavailable" {
					t.Fatal("failed request became success")
				}
			} else if reply.Type != "pending" || reply.Error != "" {
				t.Fatal("late controller lost the first read")
			}
		})
	}
}
