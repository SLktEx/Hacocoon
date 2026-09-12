package controlapi

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
	"io"
	"net"
	"time"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/logging"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

const MethodSetup = "system.setup"
const setupTimeout = 15 * time.Minute

type setupService interface {
	SetupHost(context.Context, recipes.Update) error
}

// RegisterSetup keeps bootstrap under the same controller authority as normal
// operations. There is no second local composition path in the product client.
func RegisterSetup(server *control.Server, service setupService) error {
	if server == nil || service == nil {
		return control.ErrInvalidArgument
	}
	active := make(chan struct{}, 1)
	run := func(ctx context.Context, payload json.RawMessage, report func(setupFrame)) (any, error) {
		update, err := decodeSetupUpdate(payload)
		if err != nil {
			return nil, err
		}
		select {
		case active <- struct{}{}:
			defer func() { <-active }()
		default:
			return nil, control.NewStatusError("busy", "Host setup is already running")
		}
		ctx, cancel := context.WithTimeout(ctx, setupTimeout)
		defer cancel()
		var id [16]byte
		if _, err := rand.Read(id[:]); err != nil {
			return nil, control.NewStatusError("setup_failed", "Cannot start setup diagnostics")
		}
		requestID := hex.EncodeToString(id[:])
		ctx = logging.With(ctx, "component", "bootstrap", "operation", "setup", "request_id", requestID)
		ctx = hostsetup.Observe(ctx, func(e hostsetup.Event) {
			logging.FromContext(ctx).InfoContext(ctx, "Host setup stage", "stage", e.Stage, "state", e.State, "reason", e.Reason, "duration_ms", e.DurationMS)
			if report != nil {
				report(setupFrame{Event: &e, RequestID: requestID})
			}
		})
		err = hostsetup.Step(ctx, "setup", func() error { return service.SetupHost(ctx, update) })
		if err != nil {
			stage, reason := hostsetup.Details(err)
			message, code := "Trusted Host setup failed", "setup_failed"
			if errors.Is(err, recipes.ErrExecutionFailed) {
				message, code = "Trusted Host customization failed", "customization_failed"
			}
			logging.FromContext(ctx).ErrorContext(ctx, message, "stage", stage, "reason", reason)
			return nil, control.NewStatusError(code, fmt.Sprintf("Host setup failed: stage=%s reason=%s", stage, reason))
		}
		return PingResponse{ProtocolVersion: control.ProtocolVersion}, nil
	}
	if err := server.Register(MethodSetup, func(ctx context.Context, payload json.RawMessage) (any, error) { return run(ctx, payload, nil) }); err != nil {
		return err
	}
	return server.RegisterStream(MethodSetupProgress, func(_ context.Context, payload json.RawMessage) (control.Stream, error) {
		if _, err := decodeSetupUpdate(payload); err != nil {
			return nil, err
		}
		return func(ctx context.Context, conn net.Conn) error {
			// Like the existing lifecycle RPC, disconnect does not release the setup
			// exclusion or imply rollback. The bounded operation continues in the journal.
			encoder := json.NewEncoder(conn)
			var writeErr error
			send := func(frame setupFrame) {
				if writeErr != nil {
					return
				}
				writeErr = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
				if writeErr == nil {
					writeErr = encoder.Encode(frame)
				}
			}
			_, err := run(ctx, payload, send)
			code := ""
			if err != nil {
				code = "setup_failed"
				var status *control.StatusError
				if errors.As(err, &status) {
					code = status.Code
				}
			}
			send(setupFrame{Done: true, Code: code})
			return writeErr
		}, nil
	})
}

func decodeSetupUpdate(payload json.RawMessage) (recipes.Update, error) {
	var update recipes.Update
	if len(bytes.TrimSpace(payload)) != 0 {
		decoder := json.NewDecoder(bytes.NewReader(payload))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&update); err != nil {
			return update, control.NewStatusError("invalid_argument", "invalid Host setup options")
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			return update, control.NewStatusError("invalid_argument", "invalid Host setup options")
		}
	}
	if err := update.Validate(); err != nil {
		return update, control.NewStatusError("invalid_argument", "invalid Host setup options")
	}
	return update, nil
}

const MethodSetupProgress = "system.setup.progress"

type setupFrame struct {
	Event     *hostsetup.Event `json:"event,omitempty"`
	RequestID string           `json:"request_id,omitempty"`
	Done      bool             `json:"done,omitempty"`
	Code      string           `json:"code,omitempty"`
}

// SetupHostProgress never falls back to another mutation after a stream failure.
// A bounded stream needs an explicit final acknowledgement; EOF is not success.
func (c *Client) SetupHostProgress(ctx context.Context, update recipes.Update, report func(string, hostsetup.Event)) error {
	conn, err := c.wire.OpenStream(ctx, MethodSetupProgress, update)
	if err != nil {
		return err
	}
	defer conn.Close()
	decoder := json.NewDecoder(io.LimitReader(conn, 256<<10))
	decoder.DisallowUnknownFields()
	setupComplete := false
	activeStages := map[string]int{}
	started := false
	var requestID string
	for n := 0; n < 512; n++ {
		var f setupFrame
		if err := decoder.Decode(&f); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return control.ErrProtocol
		}
		if f.Done {
			if f.Event != nil || f.RequestID != "" {
				return control.ErrProtocol
			}
			switch f.Code {
			case "":
				if !setupComplete {
					return control.ErrProtocol
				}
				return nil
			case "busy", "setup_failed", "customization_failed", "invalid_argument":
				return control.NewStatusError(f.Code, "Host setup did not complete")
			default:
				return control.ErrProtocol
			}
		}
		if f.Event == nil || f.Code != "" || !hostsetup.ValidStage(f.Event.Stage) || !hostsetup.ValidReason(f.Event.Reason) || f.Event.DurationMS < 0 {
			return control.ErrProtocol
		}
		if len(f.RequestID) != 32 {
			return control.ErrProtocol
		}
		if _, err := hex.DecodeString(f.RequestID); err != nil {
			return control.ErrProtocol
		}
		if requestID != "" && requestID != f.RequestID {
			return control.ErrProtocol
		}
		requestID = f.RequestID
		if !started && (f.Event.Stage != "setup" || f.Event.State != "running") {
			return control.ErrProtocol
		}
		if setupComplete {
			return control.ErrProtocol
		}
		if f.Event.State == "running" {
			if f.Event.Stage == "setup" && started {
				return control.ErrProtocol
			}
			started = true
			activeStages[f.Event.Stage]++
		} else {
			if activeStages[f.Event.Stage] == 0 {
				return control.ErrProtocol
			}
			activeStages[f.Event.Stage]--
			if f.Event.Stage == "setup" {
				for _, count := range activeStages {
					if count != 0 {
						return control.ErrProtocol
					}
				}
			}
		}
		switch f.Event.State {
		case "running", "succeeded":
			if f.Event.Reason != "" {
				return control.ErrProtocol
			}
		case "failed":
			if f.Event.Reason == "" {
				return control.ErrProtocol
			}
		default:
			return control.ErrProtocol
		}
		if f.Event.Stage == "setup" {
			setupComplete = f.Event.State == "succeeded"
		}
		if report != nil {
			report(requestID, *f.Event)
		}
	}
	return control.ErrProtocol
}

func (c *Client) SetupHost(ctx context.Context, update recipes.Update) error {
	var response PingResponse
	if err := c.wire.Call(ctx, MethodSetup, update, &response); err != nil {
		return err
	}
	if response.ProtocolVersion != control.ProtocolVersion {
		return control.ErrProtocol
	}
	return nil
}
