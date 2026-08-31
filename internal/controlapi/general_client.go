package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

func (c *Client) ListBases(ctx context.Context) ([]core.BaseInfo, error) {
	var response []core.BaseInfo
	err := c.wire.Call(ctx, MethodBaseList, nil, &response)
	if response == nil && err == nil {
		response = []core.BaseInfo{}
	}
	return response, err
}

func (c *Client) InspectBase(ctx context.Context, name core.BaseName) (core.BaseInfo, error) {
	var response core.BaseInfo
	err := c.wire.Call(ctx, MethodBaseInspect, BaseInspectRequest{Name: name}, &response)
	return response, err
}

func (c *Client) Run(ctx context.Context, spec runapp.Spec) (runapp.Result, error) {
	var response runResponse
	if err := c.wire.Call(ctx, MethodRun, spec, &response); err != nil {
		return runapp.Result{}, err
	}
	return response.Result, responseError(response.Error)
}

func (c *Client) RequestCapability(
	ctx context.Context,
	request core.CapabilityRequest,
	approve func(context.Context, core.ApprovalRequest) (bool, error),
) (core.CapabilityResult, error) {
	conn, err := c.wire.OpenStream(ctx, MethodCapabilityRequest, capabilityPayload(request))
	if err != nil {
		return core.CapabilityResult{}, err
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	for {
		var frame capabilityServerFrame
		if err := decoder.Decode(&frame); err != nil {
			if errors.Is(err, io.EOF) {
				return core.CapabilityResult{}, fmt.Errorf("capability stream ended without result frame: %w", control.ErrProtocol)
			}
			return core.CapabilityResult{}, err
		}
		switch frame.Type {
		case capabilityFrameApproval:
			if frame.Approval == nil || frame.Result != nil || frame.Error != nil {
				return core.CapabilityResult{}, fmt.Errorf("invalid capability approval frame: %w", control.ErrProtocol)
			}
			approved := false
			if approve != nil {
				approved, err = approve(ctx, frame.Approval.coreRequest())
				if err != nil {
					return core.CapabilityResult{}, err
				}
			}
			// A client without an approval terminal explicitly responds false.
			// This lets the controller audit an ordinary denial instead of turning
			// a missing UI callback into a transport failure.
			if err := encoder.Encode(capabilityClientFrame{Type: capabilityFrameApprovalResponse, Approved: approved}); err != nil {
				return core.CapabilityResult{}, err
			}
		case capabilityFrameResult:
			if frame.Result == nil || frame.Approval != nil {
				return core.CapabilityResult{}, fmt.Errorf("invalid capability result frame: %w", control.ErrProtocol)
			}
			return *frame.Result, responseError(frame.Error)
		default:
			return core.CapabilityResult{}, fmt.Errorf("unexpected capability server frame %q: %w", frame.Type, control.ErrProtocol)
		}
	}
}

func (c *Client) StreamEvents(ctx context.Context, sinceOffset int64, emit func(eventsapp.Event) error) (int64, error) {
	if emit == nil || sinceOffset < 0 {
		return sinceOffset, control.ErrInvalidArgument
	}
	conn, err := c.wire.OpenStream(ctx, MethodEventsStream, EventsStreamRequest{SinceOffset: sinceOffset})
	if err != nil {
		return sinceOffset, err
	}
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	nextOffset := sinceOffset
	for {
		var frame eventStreamFrame
		if err := decoder.Decode(&frame); err != nil {
			if errors.Is(err, io.EOF) {
				return nextOffset, fmt.Errorf("events stream ended without terminal frame: %w", control.ErrProtocol)
			}
			return nextOffset, err
		}
		if frame.NextOffset < 0 {
			return nextOffset, fmt.Errorf("events stream returned negative offset: %w", control.ErrProtocol)
		}
		if frame.Error != nil {
			return frame.NextOffset, responseError(frame.Error)
		}
		if frame.Event != nil {
			if frame.Done {
				return nextOffset, fmt.Errorf("events stream frame cannot contain event and done: %w", control.ErrProtocol)
			}
			nextOffset = frame.NextOffset
			if frame.Event.NextOffset != nextOffset {
				return nextOffset, fmt.Errorf("events stream offset mismatch: %w", control.ErrProtocol)
			}
			if err := emit(*frame.Event); err != nil {
				return nextOffset, err
			}
			continue
		}
		if frame.Done {
			if frame.NextOffset < nextOffset {
				return nextOffset, fmt.Errorf("events stream terminal offset moved backwards: %w", control.ErrProtocol)
			}
			return frame.NextOffset, nil
		}
		return nextOffset, fmt.Errorf("events stream returned empty frame: %w", control.ErrProtocol)
	}
}

type responseExitError struct {
	err      error
	exitCode int
}

func (e *responseExitError) Error() string { return e.err.Error() }
func (e *responseExitError) Unwrap() error { return e.err }
func (e *responseExitError) ExitCode() int { return e.exitCode }

func responseError(status *responseStatus) error {
	if status == nil {
		return nil
	}
	err := control.NewStatusError(status.Code, status.Message)
	if status.ExitCode > 0 {
		return &responseExitError{err: err, exitCode: status.ExitCode}
	}
	return err
}
