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
	for scanner.Scan() {
		line++
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var audit core.CapabilityAuditEvent
		if err := json.Unmarshal(scanner.Bytes(), &audit); err != nil {
			return nil, fmt.Errorf("decode capability audit line %d: %w", line, err)
		}
		if audit.Time.IsZero() || audit.Type == "" {
			return nil, fmt.Errorf("capability audit line %d is incomplete", line)
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
		return nil, fmt.Errorf("read capability audit: %w", err)
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
