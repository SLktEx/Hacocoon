//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var flushOperationKey = windows.NewLazySystemDLL("advapi32.dll").NewProc("RegFlushKey")
var errOperationNeedsReview = errors.New("previous reclamation is incomplete or failed; retain its record for explicit review")

// This is the last operation record, not authority or a replay plan. No commands,
// credentials, subprocess output or snapshot data belong here.
type operationRecord struct {
	Version      int
	Operation    windows.GUID
	Registration registration
	Disk         diskIdentity
	State        string
	Observation  continuationObservation
}

type operationStore struct{ key registry.Key }

func openOperationStore(id windows.GUID) (*operationStore, error) {
	if id == (windows.GUID{}) {
		return nil, errors.New("invalid operation registration")
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Hacocoon\Reclamation\`+id.String(), registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return nil, err
	}
	return &operationStore{key: key}, nil
}

func (s *operationStore) close() error { return s.key.Close() }

func (r operationRecord) validate() error {
	if r.Version != 1 || r.Operation == (windows.GUID{}) || (r.Disk.High == 0 && r.Disk.Low == 0) {
		return errors.New("unsupported reclamation record identity/version")
	}
	if _, err := r.Registration.diskPath(); err != nil {
		return err
	}
	o := r.Observation
	if o.StopRequested && !o.StopAttempted || o.Resumed && !o.ResumeAttempted || o.Compaction.Completed && !o.Compaction.Attempted || o.Compaction.Attempted && !o.StopRequested || o.Compaction.OpenAttempts < 0 {
		return errors.New("inconsistent reclamation observations")
	}
	switch r.State {
	case "pending":
		if o != (continuationObservation{}) {
			return errors.New("pending record has results")
		}
	case "complete":
		if !o.StopAttempted || !o.StopRequested || !o.Compaction.Attempted || !o.Compaction.Completed || !o.ResumeAttempted || !o.Resumed {
			return errors.New("incomplete reclamation cannot be recorded as success")
		}
	case "failed":
	default:
		return errors.New("unsupported reclamation state")
	}
	return nil
}

func (s *operationStore) read() (operationRecord, error) {
	var r operationRecord
	// Fixed allocation even for a hostile registry value. Require our canonical
	// encoding, rejecting duplicate/unknown fields rather than silently dropping them.
	data := make([]byte, 16384)
	n, kind, err := s.key.GetValue("Operation", data)
	if err != nil {
		return r, err
	}
	if kind != registry.BINARY || n < 1 || n > len(data) {
		return r, errors.New("unsupported reclamation record format")
	}
	data = data[:n]
	if err := json.Unmarshal(data, &r); err != nil {
		return operationRecord{}, errors.New("invalid reclamation record encoding")
	}
	canonical, err := json.Marshal(r)
	if err != nil || !bytes.Equal(data, canonical) {
		return operationRecord{}, errors.New("noncanonical or unknown reclamation record fields")
	}
	return r, r.validate()
}

func (s *operationStore) write(r operationRecord) error {
	if err := r.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(data) > 16384 {
		return errors.New("reclamation record too large")
	}
	if err := s.key.SetBinaryValue("Operation", data); err != nil {
		return err
	}
	// Shutdown follows only after the intent reaches the Windows registry store.
	status, _, _ := flushOperationKey.Call(uintptr(s.key))
	if status != 0 {
		return fmt.Errorf("persist reclamation record: %w", syscall.Errno(status))
	}
	current, err := s.read()
	if err != nil {
		return err
	}
	if current != r {
		return errors.New("reclamation record changed during persistence")
	}
	return nil
}

// Caller holds the registration's continuation guard. Completed observations may
// be replaced by a new intent; pending/failed/unknown records remain untouched.
func (s *operationStore) begin(r registration, disk diskIdentity) (operationRecord, error) {
	old, err := s.read()
	if err == nil {
		if old.State != "complete" {
			return operationRecord{}, errOperationNeedsReview
		}
		if old.Registration != r || old.Disk != disk {
			return operationRecord{}, errors.New("record belongs to a different registration or disk")
		}
	} else if !errors.Is(err, registry.ErrNotExist) {
		return operationRecord{}, err
	}
	id, err := windows.GenerateGUID()
	if err != nil {
		return operationRecord{}, err
	}
	record := operationRecord{Version: 1, Operation: id, Registration: r, Disk: disk, State: "pending"}
	return record, s.write(record)
}

func (s *operationStore) finish(intent operationRecord, observation continuationObservation, operationErr error) error {
	current, err := s.read()
	if err != nil {
		return err
	}
	if current != intent || intent.State != "pending" {
		return errors.New("reclamation intent changed; refusing to replace it")
	}
	result := intent
	result.Observation = observation
	result.State = "failed"
	if operationErr == nil {
		result.State = "complete"
	}
	return s.write(result)
}
