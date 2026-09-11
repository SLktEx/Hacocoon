//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

// Field order matches the installed helper's canonical encoding. This identity
// is observed from the selected Physical Host; it is not a grant of authority.
type installationIdentity struct {
	InstallationID string `json:"installation_id"`
	RegistrationID string `json:"registration_id"`
	SchemaVersion  int    `json:"schema_version"`
}

type installationObservation struct {
	Registration registration
	Installation installationIdentity
	Disk         diskIdentity
	WindowsOwner string
}

func decodeInstallation(data []byte, id windows.GUID) (installationIdentity, error) {
	var result installationIdentity
	if len(data) == 0 || len(data) > 4096 || id == (windows.GUID{}) {
		return result, errors.New("invalid installation observation size or target")
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return installationIdentity{}, errors.New("invalid installation observation encoding")
	}
	canonical, err := json.Marshal(result)
	if err != nil || !bytes.Equal(data, append(canonical, '\n')) {
		return installationIdentity{}, errors.New("unknown or noncanonical installation observation")
	}
	if result.SchemaVersion != 1 || result.RegistrationID != strings.ToLower(id.String()) || len(result.InstallationID) != 36 {
		return installationIdentity{}, errors.New("installation version or registration mismatch")
	}
	installation, err := windows.GUIDFromString("{" + result.InstallationID + "}")
	if err != nil || installation == (windows.GUID{}) || strings.ToLower(installation.String()) != "{"+result.InstallationID+"}" {
		return installationIdentity{}, errors.New("invalid installation identity")
	}
	return result, nil
}

type registrationOutput struct {
	data     []byte
	overflow bool
}

func (o *registrationOutput) Write(data []byte) (int, error) {
	if o.overflow || len(data) > 4096-len(o.data) {
		o.overflow = true
		return 0, errors.New("installation observation exceeded output limit")
	}
	o.data = append(o.data, data...)
	return len(data), nil
}

func (r registration) readInstallation(ctx context.Context) (installationIdentity, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var output registrationOutput
	if err := r.runWSLTo(ctx, wslReadRegistration, &output); err != nil {
		return installationIdentity{}, err
	}
	if output.overflow {
		return installationIdentity{}, errors.New("installation observation exceeded output limit")
	}
	return decodeInstallation(output.data, r.ID)
}

// observeInstallation holds the same exclusion as mutation so the read cannot
// restart WSL during compaction. File/ancestor pins stay held across the query.
// Enrollment must separately persist and enforce this correspondence; observing
// the current tuple alone must never enroll or authorize a replacement target.
func (r registration) observeInstallation(ctx context.Context) (installationObservation, error) {
	return r.withInstallation(ctx, nil)
}

func (r registration) withInstallation(ctx context.Context, visit func(installationObservation) error) (result installationObservation, err error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := r.revalidate(); err != nil {
		return result, err
	}
	guard, err := acquireContinuation(r.ID)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, guard.Close()) }()
	path, err := r.diskPath()
	if err != nil {
		return result, err
	}
	pin, err := pinDisk(path)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, pin.Close()) }()
	identity, err := r.readInstallation(ctx)
	if err != nil {
		return result, err
	}
	if _, err := pin.Allocation(); err != nil {
		return result, err
	}
	if err := r.revalidate(); err != nil {
		return result, err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return result, err
	}
	result = installationObservation{Registration: r, Installation: identity, Disk: pin.identity, WindowsOwner: user.User.Sid.String()}
	if visit != nil {
		return result, visit(result)
	}
	return result, nil
}
