package control

import (
	"encoding/json"
	"errors"
	"fmt"
)

const ProtocolVersion = 1

var (
	ErrInvalidArgument = errors.New("invalid control argument")
	ErrProtocol        = errors.New("control protocol error")
	ErrUnavailable     = errors.New("control endpoint unavailable")
	ErrAlreadyRunning  = errors.New("control endpoint already running")
)

type requestEnvelope struct {
	Version int             `json:"version"`
	Method  string          `json:"method"`
	Stream  bool            `json:"stream,omitempty"`
	Session bool            `json:"session,omitempty"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type responseEnvelope struct {
	Version   int             `json:"version"`
	SessionID string          `json:"session_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Error     *wireError      `json:"error,omitempty"`
}

type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type StatusError struct {
	Code    string
	Message string
}

func (e *StatusError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewStatusError(code, message string) error {
	return &StatusError{Code: code, Message: message}
}

func errorEnvelope(err error) responseEnvelope {
	if err == nil {
		return responseEnvelope{Version: ProtocolVersion}
	}
	status := &StatusError{Code: "internal", Message: err.Error()}
	var typed *StatusError
	if errors.As(err, &typed) {
		status = typed
	}
	return responseEnvelope{
		Version: ProtocolVersion,
		Error: &wireError{
			Code:    status.Code,
			Message: status.Message,
		},
	}
}

func validateResponse(response responseEnvelope) error {
	if response.Version != ProtocolVersion {
		return fmt.Errorf("unsupported control protocol version %d: %w", response.Version, ErrProtocol)
	}
	if response.Error != nil {
		return &StatusError{Code: response.Error.Code, Message: response.Error.Message}
	}
	return nil
}
