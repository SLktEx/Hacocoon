package events

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Event struct {
	RequestID   string              `json:"request_id,omitempty"`
	Time        time.Time           `json:"time"`
	Source      string              `json:"source"`
	Type        string              `json:"type"`
	Environment string              `json:"environment,omitempty"`
	Capability  string              `json:"capability,omitempty"`
	Action      string              `json:"action,omitempty"`
	Resource    string              `json:"resource,omitempty"`
	Attributes  map[string]string   `json:"attributes,omitempty"`
	Decision    core.PolicyDecision `json:"decision,omitempty"`
	Approved    *bool               `json:"approved,omitempty"`
	Success     *bool               `json:"success,omitempty"`
	Reason      string              `json:"reason,omitempty"`
}

type CorruptionKind string

const (
	CorruptionMalformedJSON CorruptionKind = "malformed-json"
	CorruptionIncomplete    CorruptionKind = "incomplete-record"
	CorruptionReadError     CorruptionKind = "read-error"
)

// AuditCorruptionError reports the first audit record that cannot be trusted.
// List returns only the valid prefix before this position; records at and after
// the corruption are deliberately not exposed because their ordering/history
// can no longer be established from the JSONL stream.
type AuditCorruptionError struct {
	Line       int
	ByteOffset int64
	Kind       CorruptionKind
	Err        error
}

func (e *AuditCorruptionError) Error() string {
	if e == nil {
		return "capability audit corruption"
	}
	if e.Err != nil {
		return fmt.Sprintf("capability audit corruption at line %d byte %d (%s): %v", e.Line, e.ByteOffset, e.Kind, e.Err)
	}
	return fmt.Sprintf("capability audit corruption at line %d byte %d (%s)", e.Line, e.ByteOffset, e.Kind)
}

func (e *AuditCorruptionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type Service struct {
	path string
}

func New(path string) *Service { return &Service{path: path} }

func (s *Service) List(ctx context.Context) ([]Event, error) {
	if s == nil || s.path == "" {
		return nil, core.ErrInvalidArgument
	}
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return []Event{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open capability audit: %w", err)
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	line := 0
	var byteOffset int64
	for scanner.Scan() {
		line++
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		recordOffset := byteOffset
		record := scanner.Bytes()
		// Scanner's line split removes '\n' but preserves a preceding '\r'.
		// Every record followed by another record therefore advances by the
		// scanned bytes plus the one removed newline byte.
		byteOffset += int64(len(record)) + 1

		var audit core.CapabilityAuditEvent
		if err := json.Unmarshal(record, &audit); err != nil {
			return events, &AuditCorruptionError{
				Line:       line,
				ByteOffset: recordOffset,
				Kind:       CorruptionMalformedJSON,
				Err:        err,
			}
		}
		if audit.Time.IsZero() || audit.Type == "" {
			return events, &AuditCorruptionError{
				Line:       line,
				ByteOffset: recordOffset,
				Kind:       CorruptionIncomplete,
				Err:        errors.New("required time/type field is missing"),
			}
		}
		events = append(events, Event{
			RequestID:   audit.RequestID,
			Time:        audit.Time,
			Source:      "capability",
			Type:        audit.Type,
			Environment: audit.Environment,
			Capability:  audit.Capability,
			Action:      audit.Action,
			Resource:    audit.Resource,
			Attributes:  clone(audit.Attributes),
			Decision:    audit.Decision,
			Approved:    audit.Approved,
			Success:     audit.Success,
			Reason:      audit.Reason,
		})
	}
	if err := scanner.Err(); err != nil {
		return events, &AuditCorruptionError{
			Line:       line + 1,
			ByteOffset: byteOffset,
			Kind:       CorruptionReadError,
			Err:        err,
		}
	}
	return events, nil
}

func clone(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
