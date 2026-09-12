package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/SLktEx/Hacocoon/pkg/interaction"
)

const failureNoticeInterval = time.Minute
const maxFailureNotices = 64

// Presentation state only: audit events and approval requests are never removed.
// Hash the minimized tuple to keep the persisted record bounded and unambiguous.
type failureNotice struct {
	Key string    `json:"key"`
	At  time.Time `json:"at"`
}

func failureNoticeKey(event interaction.Event) string {
	if event.RecoveryRequired {
		return ""
	}
	switch event.Kind {
	case interaction.OperationFailed, interaction.PolicyDenied, interaction.ApprovalDenied:
	default:
		return ""
	}
	data, _ := json.Marshal([5]string{string(event.Kind), event.Environment, event.Capability, event.Action, event.Code})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *notifyState) suppressFailure(event interaction.Event, now time.Time) bool {
	key := failureNoticeKey(event)
	if key == "" {
		return false
	}
	for _, notice := range s.Failures {
		elapsed := now.Sub(notice.At)
		// A clock rollback cannot hide a failure indefinitely.
		if notice.Key == key && elapsed >= 0 && elapsed < failureNoticeInterval {
			return true
		}
	}
	return false
}

func (s *notifyState) rememberFailure(event interaction.Event, now time.Time) {
	key := failureNoticeKey(event)
	if key == "" {
		return
	}
	kept := make([]failureNotice, 0, maxFailureNotices)
	for _, notice := range s.Failures {
		elapsed := now.Sub(notice.At)
		if notice.Key != key && elapsed >= 0 && elapsed < failureNoticeInterval {
			kept = append(kept, notice)
			if len(kept) == maxFailureNotices-1 {
				break
			}
		}
	}
	s.Failures = append(kept, failureNotice{Key: key, At: now})
}
