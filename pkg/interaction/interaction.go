package interaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	eventsapp "github.com/SLktEx/Hacocoon/internal/events"
)

const (
	SchemaVersion    = 1
	DefaultBatchSize = 100
	MaxBatchSize     = 1000
)

type Kind string

const (
	ApprovalRequired   Kind = "approval-required"
	ApprovalApproved   Kind = "approval-approved"
	ApprovalDenied     Kind = "approval-denied"
	PolicyDenied       Kind = "policy-denied"
	OperationCompleted Kind = "operation-completed"
	OperationFailed    Kind = "operation-failed"
	RecoveryRequired   Kind = "recovery-required"
)

type Event struct {
	SchemaVersion     int       `json:"schema_version"`
	EventID           string    `json:"event_id"`
	RequestID         string    `json:"request_id"`
	Time              time.Time `json:"time"`
	Kind              Kind      `json:"kind"`
	Environment       string    `json:"environment,omitempty"`
	Capability        string    `json:"capability,omitempty"`
	Action            string    `json:"action,omitempty"`
	Code              string    `json:"code,omitempty"`
	RequiresAttention bool      `json:"requires_attention"`
	RecoveryRequired  bool      `json:"recovery_required"`
	NextOffset        int64     `json:"next_offset"`
}

type Batch struct {
	SchemaVersion int     `json:"schema_version"`
	Events        []Event `json:"events"`
	NextOffset    int64   `json:"next_offset"`
}

type Reader struct {
	events *eventsapp.Service
}

func NewReader(root string) (*Reader, error) {
	root = filepath.Clean(root)
	if root == "." || !filepath.IsAbs(root) {
		return nil, ErrInvalidArgument
	}
	return &Reader{events: eventsapp.New(filepath.Join(root, "audit", "capabilities.jsonl"))}, nil
}

func NewDefaultReader() (*Reader, error) {
	root := os.Getenv("HACO_ROOT")
	if root == "" {
		root = "/var/lib/hacocoon"
	}
	return NewReader(root)
}

func (r *Reader) Stream(ctx context.Context, offset int64, emit func(Event) error) (int64, error) {
	if r == nil || r.events == nil || emit == nil || offset < 0 {
		return offset, ErrInvalidArgument
	}
	next, err := r.events.Stream(ctx, offset, func(raw eventsapp.Event) error {
		event, ok, err := project(raw)
		if err != nil || !ok {
			return err
		}
		return emit(event)
	})
	return next, translateError(err)
}

var (
	ErrInvalidArgument = errors.New("invalid interaction argument")
	ErrInvalidEvent    = errors.New("invalid interaction source event")
	errBatchFull       = errors.New("interaction batch full")
)

type CorruptionKind string

const (
	CorruptionMalformedJSON CorruptionKind = "malformed-json"
	CorruptionIncomplete    CorruptionKind = "incomplete-record"
	CorruptionReadError     CorruptionKind = "read-error"
)

type CorruptionError struct {
	Line       int            `json:"line"`
	ByteOffset int64          `json:"byte_offset"`
	Kind       CorruptionKind `json:"kind"`
}

func (e *CorruptionError) Error() string {
	if e == nil {
		return "interaction source corruption"
	}
	return fmt.Sprintf("interaction source corruption at line %d byte %d (%s)", e.Line, e.ByteOffset, e.Kind)
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	var corruption *eventsapp.AuditCorruptionError
	if errors.As(err, &corruption) {
		return &CorruptionError{Line: corruption.Line, ByteOffset: corruption.ByteOffset, Kind: CorruptionKind(corruption.Kind)}
	}
	if errors.Is(err, core.ErrInvalidArgument) {
		return ErrInvalidArgument
	}
	return err
}

func (r *Reader) Batch(ctx context.Context, offset int64, limit int) (Batch, error) {
	batch := Batch{SchemaVersion: SchemaVersion, Events: []Event{}, NextOffset: offset}
	if r == nil || r.events == nil || offset < 0 || limit < 1 || limit > MaxBatchSize {
		return batch, ErrInvalidArgument
	}

	lastEmittedOffset := offset
	streamOffset, err := r.events.Stream(ctx, offset, func(raw eventsapp.Event) error {
		event, ok, projectionErr := project(raw)
		if projectionErr != nil {
			return projectionErr
		}
		if !ok {
			return nil
		}
		batch.Events = append(batch.Events, event)
		lastEmittedOffset = event.NextOffset
		if len(batch.Events) == limit {
			return errBatchFull
		}
		return nil
	})
	if errors.Is(err, errBatchFull) {
		batch.NextOffset = lastEmittedOffset
		return batch, nil
	}
	batch.NextOffset = streamOffset
	return batch, translateError(err)
}

func project(raw eventsapp.Event) (Event, bool, error) {
	kind, code, attention, recovery, ok := classify(raw)
	if !ok {
		return Event{}, false, nil
	}
	if raw.RequestID == "" {
		return Event{}, false, fmt.Errorf("%w: event %q has no request id", ErrInvalidEvent, raw.Type)
	}
	event := Event{
		SchemaVersion:     SchemaVersion,
		EventID:           raw.RequestID + ":" + string(kind),
		RequestID:         raw.RequestID,
		Time:              raw.Time.UTC(),
		Kind:              kind,
		Environment:       raw.Environment,
		Capability:        raw.Capability,
		Action:            raw.Action,
		Code:              code,
		RequiresAttention: attention,
		RecoveryRequired:  recovery,
		NextOffset:        raw.NextOffset,
	}
	return event, true, nil
}

func classify(raw eventsapp.Event) (Kind, string, bool, bool, bool) {
	switch raw.Type {
	case "policy-decision":
		switch raw.Decision {
		case core.PolicyRequireApproval:
			return ApprovalRequired, "", true, false, true
		case core.PolicyDeny:
			return PolicyDenied, "policy-denied", true, false, true
		default:
			return "", "", false, false, false
		}
	case "policy-error":
		return PolicyDenied, safeCode(raw.Reason, "policy-error"), true, false, true
	case "approval-decision":
		if raw.Approved == nil {
			return "", "", false, false, false
		}
		if *raw.Approved {
			return ApprovalApproved, "", false, false, true
		}
		return ApprovalDenied, "approval-denied", true, false, true
	case "completed":
		if raw.Success == nil {
			return "", "", false, false, false
		}
		if *raw.Success {
			return OperationCompleted, "", false, false, true
		}
		code := safeCode(raw.Reason, "operation-failed")
		if code == "recovery-required" || code == "incompatible-state" {
			return RecoveryRequired, code, true, true, true
		}
		return OperationFailed, code, true, false, true
	default:
		return "", "", false, false, false
	}
}

func safeCode(reason, fallback string) string {
	switch reason {
	case "policy-evaluation-failed",
		"invalid-policy-decision",
		"approval-provider-failed",
		"context-canceled",
		"context-deadline-exceeded",
		"capability-stale",
		"invalid-argument",
		"not-found",
		"already-exists",
		"unsupported",
		"unsafe-shrink",
		"runtime-unavailable",
		"storage-unavailable",
		"storage-busy",
		"workspace-busy",
		"policy-denied",
		"approval-denied",
		"audit-incomplete",
		"incompatible-state",
		"recovery-required",
		"provider-execution-failed":
		return reason
	default:
		return fallback
	}
}
