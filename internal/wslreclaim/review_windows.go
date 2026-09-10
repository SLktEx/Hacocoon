//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"context"
	"encoding/json"
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func failedReviewName(id windows.GUID) string { return "ReviewedFailed-" + id.String() }

// The original version-1 result is retained verbatim, not rewritten as success.
// This is an explicit acknowledgement of a terminal failure, never crash replay.
func (s *operationStore) reviewFailed(id windows.GUID, r registration, disk diskIdentity) error {
	current, err := s.read()
	if err != nil {
		return err
	}
	if id == (windows.GUID{}) || current.Operation != id || current.Registration != r || current.Disk != disk || current.State != "failed" {
		return errors.New("review requires the exact failed operation and enrolled target")
	}
	name := failedReviewName(id)
	prior, err := s.readValue(name)
	if err == nil {
		if prior != current {
			return errors.New("existing failed evidence differs; retain both records")
		}
	} else if errors.Is(err, registry.ErrNotExist) {
		raw, err := json.Marshal(current)
		if err != nil {
			return err
		}
		if err := s.key.SetBinaryValue(name, raw); err != nil {
			return err
		}
	} else {
		return err
	}
	// Flush even on the idempotent path: a previous flush may have failed.
	status, _, _ := flushOperationKey.Call(uintptr(s.key))
	if status != 0 {
		return syscall.Errno(status)
	}
	saved, err := s.readValue(name)
	if err != nil || saved != current {
		return errors.New("failed evidence persistence unconfirmed")
	}
	latest, err := s.read()
	if err != nil || latest != current {
		return errors.New("operation changed during review")
	}
	return nil
}

// Caller supplies only exact GUIDs. The existing guard, enrollment, Windows user,
// Host identity and held disk checks apply before any acknowledgement is stored.
func ReviewFailedOperation(ctx context.Context, registrationID, operationID string) error {
	rID, id, err := preparedIDs(registrationID, operationID)
	if err != nil {
		return err
	}
	r, err := readRegistration(rID.String())
	if err != nil {
		return err
	}
	return r.withReclamationTarget(ctx, func(records *operationStore, pin *pinnedDisk, _ installationObservation) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return records.reviewFailed(id, r, pin.identity)
	})
}

// Status remains available by exact operation ID after a newer attempt begins.
// Malformed current data is never silently ignored in favor of older evidence.
func (s *operationStore) readOperation(id windows.GUID) (operationRecord, error) {
	current, err := s.read()
	if err == nil && current.Operation == id {
		return current, nil
	}
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return operationRecord{}, err
	}
	saved, err := s.readValue(failedReviewName(id))
	if err != nil {
		return operationRecord{}, err
	}
	if saved.Operation != id || saved.State != "failed" {
		return operationRecord{}, errors.New("invalid reviewed failure evidence")
	}
	return saved, nil
}
