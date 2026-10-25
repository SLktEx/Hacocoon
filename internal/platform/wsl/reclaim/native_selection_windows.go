//go:build windows && (amd64 || arm64)

package wslreclaim

import (
	"errors"
	"fmt"
	"io"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// ResolveNativeRegistration resolves only an explicitly named, already enrolled
// registration. It neither starts WSL nor creates registry keys. A display name
// is selection, not authority: preparation repeats the complete disk/owner and
// installation checks under exclusion. Status does not pin or reopen the disk,
// which could otherwise interfere with an active compaction.
func ResolveNativeRegistration(distribution string) (_ string, err error) {
	if !nativeDistributionName.MatchString(distribution) {
		return "", errors.New("an explicit managed WSL distribution name is required")
	}
	candidates, err := nativeRegistrationNames()
	if err != nil {
		return "", err
	}
	id, err := selectNativeRegistration(distribution, candidates)
	if err != nil {
		return "", err
	}
	r, err := readRegistration(id)
	if err != nil {
		return "", err
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Hacocoon\Reclamation\`+r.ID.String(), registry.QUERY_VALUE)
	if err != nil {
		return "", fmt.Errorf("managed WSL enrollment required: %w", err)
	}
	records := &operationStore{key: key}
	defer func() { err = errors.Join(err, records.close()) }()
	binding, err := records.readBinding()
	if err != nil {
		return "", err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	if binding.Target.Registration != r || binding.Target.WindowsOwner != user.User.Sid.String() {
		return "", errors.New("named WSL registration or owner differs from enrollment")
	}
	if err := r.revalidate(); err != nil {
		return "", err
	}
	return r.ID.String(), nil
}

func nativeRegistrationNames() (_ []nativeRegistrationName, err error) {
	const lxss = `Software\Microsoft\Windows\CurrentVersion\Lxss`
	key, err := registry.OpenKey(registry.CURRENT_USER, lxss, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, key.Close()) }()
	keys, err := key.ReadSubKeyNames(maxNativeRegistrations + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(keys) > maxNativeRegistrations {
		return nil, errors.New("WSL registration enumeration exceeded limit")
	}
	var result []nativeRegistrationName
	for _, name := range keys {
		// Observe only names here: unrelated WSL 1 or nonmanaged registrations
		// must not be mistaken for enrolled candidates or require WSL access.
		path, err := registrationKey(name)
		if err != nil {
			return nil, err
		}
		candidate, err := registry.OpenKey(registry.CURRENT_USER, path, registry.QUERY_VALUE)
		if err != nil {
			return nil, err
		}
		var data [514]byte
		n, kind, readErr := candidate.GetValue("DistributionName", data[:])
		if err := errors.Join(readErr, candidate.Close()); err != nil {
			return nil, err
		}
		if kind != registry.SZ || n < 0 || n > len(data) {
			return nil, errors.New("WSL distribution name must be a literal string")
		}
		distribution, err := decodeNativeDistributionName(data[:n])
		if err != nil {
			return nil, err
		}
		result = append(result, nativeRegistrationName{id: name, name: distribution})
	}
	return result, nil
}
