package events

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const maxAuditRecordBytes = 1024 * 1024

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
	NextOffset  int64               `json:"next_offset"`
}

type CorruptionKind string

const (
	CorruptionMalformedJSON CorruptionKind = "malformed-json"
	CorruptionIncomplete    CorruptionKind = "incomplete-record"
	CorruptionReadError     CorruptionKind = "read-error"
)

// AuditCorruptionError reports the first audit record that cannot be trusted.
// ByteOffset is always the absolute byte offset in the audit file. Line is
// one-based relative to the requested stream offset; for List (offset zero) it
// is therefore the absolute file line. Records at and after the corruption are
// deliberately not exposed.
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

// Stream reads complete JSONL records beginning at offset and emits them one at
// a time. The returned offset is the next safe record boundary after the last
// successfully emitted event. Memory use is bounded by one audit record plus
// the caller's own callback state.
func (s *Service) Stream(ctx context.Context, offset int64, emit func(Event) error) (int64, error) {
	if s == nil || s.path == "" || offset < 0 || emit == nil {
		return offset, core.ErrInvalidArgument
	}
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		if offset == 0 {
			return 0, nil
		}
		return offset, fmt.Errorf("event offset %d cannot be resumed because the audit file is missing: %w", offset, core.ErrInvalidArgument)
	}
	if err != nil {
		return offset, fmt.Errorf("open capability audit: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return offset, fmt.Errorf("stat capability audit: %w", err)
	}
	if offset > info.Size() {
		return offset, fmt.Errorf("event offset %d exceeds current audit size %d (log may have been truncated or rotated): %w", offset, info.Size(), core.ErrInvalidArgument)
	}
	if offset > 0 && offset < info.Size() {
		var previous [1]byte
		if _, err := file.ReadAt(previous[:], offset-1); err != nil {
			return offset, fmt.Errorf("validate event offset %d: %w", offset, err)
		}
		if previous[0] != '\n' {
			return offset, fmt.Errorf("event offset %d is not a JSONL record boundary: %w", offset, core.ErrInvalidArgument)
		}
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return offset, fmt.Errorf("seek capability audit to %d: %w", offset, err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), maxAuditRecordBytes+2)
	scanner.Split(scanJSONLRecord)
	currentOffset := offset
	line := 0
	for scanner.Scan() {
		line++
		select {
		case <-ctx.Done():
			return currentOffset, ctx.Err()
		default:
		}

		rawRecord := scanner.Bytes()
		recordOffset := currentOffset
		currentOffset += int64(len(rawRecord))
		record := bytes.TrimSuffix(rawRecord, []byte{'\n'})
		if len(record) > maxAuditRecordBytes {
			return recordOffset, &AuditCorruptionError{
				Line:       line,
				ByteOffset: recordOffset,
				Kind:       CorruptionReadError,
				Err:        fmt.Errorf("audit record exceeds %d bytes", maxAuditRecordBytes),
			}
		}

		var audit core.CapabilityAuditEvent
		if err := json.Unmarshal(record, &audit); err != nil {
			return recordOffset, &AuditCorruptionError{
				Line:       line,
				ByteOffset: recordOffset,
				Kind:       CorruptionMalformedJSON,
				Err:        err,
			}
		}
		if audit.Time.IsZero() || audit.Type == "" {
			return recordOffset, &AuditCorruptionError{
				Line:       line,
				ByteOffset: recordOffset,
				Kind:       CorruptionIncomplete,
				Err:        errors.New("required time/type field is missing"),
			}
		}
		event := Event{
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
			NextOffset:  currentOffset,
		}
		if err := emit(event); err != nil {
			return recordOffset, err
		}
	}
	if err := scanner.Err(); err != nil {
		return currentOffset, &AuditCorruptionError{
			Line:       line + 1,
			ByteOffset: currentOffset,
			Kind:       CorruptionReadError,
			Err:        err,
		}
	}
	return currentOffset, nil
}

func (s *Service) List(ctx context.Context) ([]Event, error) {
	var events []Event
	_, err := s.Stream(ctx, 0, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if events == nil {
		events = []Event{}
	}
	return events, err
}

func scanJSONLRecord(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		return i + 1, data[:i+1], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
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
