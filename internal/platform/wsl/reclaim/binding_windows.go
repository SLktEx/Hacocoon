//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type installationBinding struct {
	Version int
	Target  installationObservation
}

func (b installationBinding) validate() error {
	if b.Version != 1 || b.Target.Disk.High == 0 && b.Target.Disk.Low == 0 {
		return errors.New("invalid installation binding version or file identity")
	}
	if _, err := b.Target.Registration.diskPath(); err != nil {
		return err
	}
	raw, err := json.Marshal(b.Target.Installation)
	if err != nil {
		return err
	}
	if _, err := decodeInstallation(append(raw, '\n'), b.Target.Registration.ID); err != nil {
		return err
	}
	sid, err := windows.StringToSid(b.Target.WindowsOwner)
	if err != nil || sid.String() != b.Target.WindowsOwner {
		return errors.New("invalid installation Windows owner")
	}
	return nil
}

func (s *operationStore) readBinding() (installationBinding, error) {
	var binding installationBinding
	data := make([]byte, 4096)
	n, kind, err := s.key.GetValue("Installation", data)
	if err != nil {
		return binding, err
	}
	if kind != registry.BINARY || n < 1 || n > len(data) {
		return binding, errors.New("invalid installation binding encoding")
	}
	data = data[:n]
	if err := json.Unmarshal(data, &binding); err != nil {
		return installationBinding{}, errors.New("malformed installation binding")
	}
	encoded, err := json.Marshal(binding)
	if err != nil || !bytes.Equal(encoded, data) {
		return installationBinding{}, errors.New("unknown or noncanonical installation binding")
	}
	return binding, binding.validate()
}

// Only explicit installation enrollment calls this method. It does not replace
// existing correspondence or acknowledge an interrupted operation.
func (s *operationStore) enrollBinding(target installationObservation) error {
	binding := installationBinding{Version: 1, Target: target}
	if err := binding.validate(); err != nil {
		return err
	}
	current, err := s.readBinding()
	if err == nil {
		if current != binding {
			return errors.New("installation binding changed; explicit review required")
		}
		return nil
	}
	if !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	data, err := json.Marshal(binding)
	if err != nil {
		return err
	}
	if len(data) > 4096 {
		return errors.New("installation binding too large")
	}
	if err := s.key.SetBinaryValue("Installation", data); err != nil {
		return err
	}
	status, _, _ := flushOperationKey.Call(uintptr(s.key))
	if status != 0 {
		return fmt.Errorf("persist installation binding: %w", syscall.Errno(status))
	}
	current, err = s.readBinding()
	if err != nil {
		return err
	}
	if current != binding {
		return errors.New("installation binding changed during persistence")
	}
	return nil
}

func (s *operationStore) requireBinding(target installationObservation) error {
	current, err := s.readBinding()
	if err != nil {
		return fmt.Errorf("managed WSL enrollment required: %w", err)
	}
	if current.Target != target {
		return errors.New("managed WSL installation/owner/disk no longer matches enrollment")
	}
	return nil
}

// Installer-only entry. Holding the observer's guard and native pins through
// persistence prevents enrollment from following a substituted file name.
// Packaging must route here only for an explicit managed installation request.
func (r registration) enrollInstallation(ctx context.Context) error {
	_, err := r.withInstallation(ctx, func(target installationObservation) (err error) {
		records, err := openOperationStore(r.ID)
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, records.close()) }()
		return records.enrollBinding(target)
	})
	return err
}

// EnrollInstallation is the native installer's explicit entry. Ordinary
// reclamation never calls this function to adopt a missing or changed target.
func EnrollInstallation(ctx context.Context, registrationID string) error {
	r, err := readRegistration(registrationID)
	if err != nil {
		return err
	}
	return r.enrollInstallation(ctx)
}
