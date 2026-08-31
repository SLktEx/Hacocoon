package controlapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/control"
	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
	runapp "github.com/SLktEx/Hacocoon/internal/run"
)

const (
	MethodBaseList          = "base.list"
	MethodBaseInspect       = "base.inspect"
	MethodRun               = "run.execute"
	MethodEventsStream      = "events.stream"
	MethodCapabilityRequest = "capability.request"

	capabilityFrameApproval         = "approval"
	capabilityFrameApprovalResponse = "approval_response"
	capabilityFrameResult           = "result"
)

type BaseInspectRequest struct {
	Name core.BaseName `json:"name"`
}

type EventsStreamRequest struct {
	SinceOffset int64 `json:"since_offset"`
}

// CapabilityRequestPayload is deliberately separate from core.CapabilityRequest.
// Core keeps Parameters out of JSON because they are non-authority execution
// inputs; the controller protocol still has to transport them explicitly after
// the client has parsed the request.
type CapabilityRequestPayload struct {
	Capability  string            `json:"capability"`
	Action      string            `json:"action"`
	Resource    string            `json:"resource,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

type ApprovalRequestPayload struct {
	Capability  string            `json:"capability"`
	Action      string            `json:"action"`
	Resource    string            `json:"resource,omitempty"`
	Environment string            `json:"environment,omitempty"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	Reason      string            `json:"reason,omitempty"`
}

type responseStatus struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	ExitCode int    `json:"exit_code,omitempty"`
}

type eventStreamFrame struct {
	Event      *eventsapp.Event `json:"event,omitempty"`
	NextOffset int64            `json:"next_offset"`
	Done       bool             `json:"done,omitempty"`
	Error      *responseStatus  `json:"error,omitempty"`
}

type runResponse struct {
	Result runapp.Result   `json:"result"`
	Error  *responseStatus `json:"error,omitempty"`
}

type capabilityServerFrame struct {
	Type     string                  `json:"type"`
	Approval *ApprovalRequestPayload `json:"approval,omitempty"`
	Result   *core.CapabilityResult  `json:"result,omitempty"`
	Error    *responseStatus         `json:"error,omitempty"`
}

type capabilityClientFrame struct {
	Type     string `json:"type"`
	Approved bool   `json:"approved"`
}

type capabilityStreamTransportError struct{ err error }

func (e *capabilityStreamTransportError) Error() string { return fmt.Sprintf("capability approval stream: %v", e.err) }
func (e *capabilityStreamTransportError) Unwrap() error { return e.err }

type baseService interface {
	ListBases(context.Context) ([]core.BaseInfo, error)
	InspectBase(context.Context, core.BaseName) (core.BaseInfo, error)
}

type runService interface {
	Run(context.Context, runapp.Spec) (runapp.Result, error)
}

type eventService interface {
	Stream(context.Context, int64, func(eventsapp.Event) error) (int64, error)
}

type capabilityService interface {
	RequestWithApproval(
		context.Context,
		core.CapabilityRequest,
		func(context.Context, core.ApprovalRequest) (bool, error),
	) (core.CapabilityResult, error)
}

// RegisterGeneral installs general-client operations that are intentionally
// separate from the initial Environment lifecycle registration. The split lets
// the controller grow typed APIs without changing the stable Environment
// registration contract used by older tests and callers.
func RegisterGeneral(server *control.Server, bases baseService, runner runService, events eventService, capabilities capabilityService) error {
	if server == nil || bases == nil || runner == nil || events == nil || capabilities == nil {
		return control.ErrInvalidArgument
	}
	if err := server.Register(MethodBaseList, func(ctx context.Context, _ json.RawMessage) (any, error) {
		result, err := bases.ListBases(ctx)
		if err != nil {
			return nil, translateError(err)
		}
		if result == nil {
			result = []core.BaseInfo{}
		}
		return result, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodBaseInspect, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request BaseInspectRequest
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(string(request.Name)) == "" {
			return nil, control.NewStatusError("invalid_argument", "base name is required")
		}
		result, err := bases.InspectBase(ctx, request.Name)
		if err != nil {
			return nil, translateError(err)
		}
		return result, nil
	}); err != nil {
		return err
	}
	if err := server.Register(MethodRun, func(ctx context.Context, payload json.RawMessage) (any, error) {
		var request runapp.Spec
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.WorkspacePath) == "" || len(request.Argv) == 0 {
			return nil, control.NewStatusError("invalid_argument", "workspace_path and argv are required")
		}
		result, err := runner.Run(ctx, request)
		if err != nil {
			// Preserve both the partial result and the original process exit code.
			// run errors may be joined with cleanup/recovery failures, so the status
			// message remains intact while ExitCode lets the CLI retain historical
			// process semantics across the controller boundary.
			return runResponse{Result: result, Error: statusFromError(err)}, nil
		}
		return runResponse{Result: result}, nil
	}); err != nil {
		return err
	}
	if err := server.RegisterStream(MethodCapabilityRequest, func(ctx context.Context, payload json.RawMessage) (control.Stream, error) {
		var request CapabilityRequestPayload
		if err := json.Unmarshal(payload, &request); err != nil || strings.TrimSpace(request.Capability) == "" || strings.TrimSpace(request.Action) == "" {
			return nil, control.NewStatusError("invalid_argument", "invalid capability request")
		}
		return func(runCtx context.Context, conn net.Conn) error {
			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)
			result, requestErr := capabilities.RequestWithApproval(runCtx, request.coreRequest(), func(approvalCtx context.Context, approval core.ApprovalRequest) (bool, error) {
				select {
				case <-approvalCtx.Done():
					return false, approvalCtx.Err()
				default:
				}
				payload := approvalPayload(approval)
				if err := encoder.Encode(capabilityServerFrame{Type: capabilityFrameApproval, Approval: &payload}); err != nil {
					return false, &capabilityStreamTransportError{err: err}
				}
				var response capabilityClientFrame
				if err := decoder.Decode(&response); err != nil {
					return false, &capabilityStreamTransportError{err: err}
				}
				if response.Type != capabilityFrameApprovalResponse {
					return false, &capabilityStreamTransportError{err: fmt.Errorf("unexpected capability client frame %q: %w", response.Type, control.ErrProtocol)}
				}
				return response.Approved, nil
			})
			var transportErr *capabilityStreamTransportError
			if errors.As(requestErr, &transportErr) {
				return transportErr
			}
			copyResult := result
			return encoder.Encode(capabilityServerFrame{Type: capabilityFrameResult, Result: &copyResult, Error: statusFromError(requestErr)})
		}, nil
	}); err != nil {
		return err
	}
	if err := server.RegisterStream(MethodEventsStream, func(ctx context.Context, payload json.RawMessage) (control.Stream, error) {
		var request EventsStreamRequest
		if err := json.Unmarshal(payload, &request); err != nil || request.SinceOffset < 0 {
			return nil, control.NewStatusError("invalid_argument", "since_offset must be non-negative")
		}
		return func(runCtx context.Context, conn net.Conn) error {
			encoder := json.NewEncoder(conn)
			writeFailed := false
			nextOffset, streamErr := events.Stream(runCtx, request.SinceOffset, func(event eventsapp.Event) error {
				copyEvent := event
				if err := encoder.Encode(eventStreamFrame{Event: &copyEvent, NextOffset: event.NextOffset}); err != nil {
					writeFailed = true
					return err
				}
				return nil
			})
			if writeFailed {
				return streamErr
			}
			if streamErr != nil {
				return encoder.Encode(eventStreamFrame{NextOffset: nextOffset, Error: statusFromError(streamErr)})
			}
			return encoder.Encode(eventStreamFrame{NextOffset: nextOffset, Done: true})
		}, nil
	}); err != nil {
		return err
	}
	return nil
}

func capabilityPayload(request core.CapabilityRequest) CapabilityRequestPayload {
	return CapabilityRequestPayload{
		Capability:  request.Capability,
		Action:      request.Action,
		Resource:    request.Resource,
		Environment: request.Environment,
		Attributes:  cloneStringMap(request.Attributes),
		Parameters:  cloneStringMap(request.Parameters),
	}
}

func (r CapabilityRequestPayload) coreRequest() core.CapabilityRequest {
	return core.CapabilityRequest{
		Capability:  r.Capability,
		Action:      r.Action,
		Resource:    r.Resource,
		Environment: r.Environment,
		Attributes:  cloneStringMap(r.Attributes),
		Parameters:  cloneStringMap(r.Parameters),
	}
}

func approvalPayload(request core.ApprovalRequest) ApprovalRequestPayload {
	return ApprovalRequestPayload{
		Capability:  request.CapabilityRequest.Capability,
		Action:      request.CapabilityRequest.Action,
		Resource:    request.CapabilityRequest.Resource,
		Environment: request.CapabilityRequest.Environment,
		Attributes:  cloneStringMap(request.CapabilityRequest.Attributes),
		Reason:      request.Reason,
	}
}

func (r ApprovalRequestPayload) coreRequest() core.ApprovalRequest {
	return core.ApprovalRequest{
		CapabilityRequest: core.CapabilityRequest{
			Capability:  r.Capability,
			Action:      r.Action,
			Resource:    r.Resource,
			Environment: r.Environment,
			Attributes:  cloneStringMap(r.Attributes),
		},
		Reason: r.Reason,
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func statusFromError(err error) *responseStatus {
	if err == nil {
		return nil
	}
	translated := translateError(err)
	status := &responseStatus{Code: "internal", Message: translated.Error()}
	var typed *control.StatusError
	if errors.As(translated, &typed) {
		status.Code = typed.Code
		status.Message = typed.Message
	}
	var exitCoder interface{ ExitCode() int }
	if errors.As(err, &exitCoder) && exitCoder.ExitCode() > 0 {
		status.ExitCode = exitCoder.ExitCode()
	}
	return status
}
