//go:build windows

package wslreclaim

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// registration is an observation, not permission to stop a distribution. The
// Windows continuation must bind an authorized managed installation to this ID.
// Never resolve a missing ID through the default distribution or a reused name.
type registration struct {
	ID                          windows.GUID
	Name, BasePath, VHDFileName string
}

func registrationKey(id string) (string, error) {
	guid, err := windows.GUIDFromString(id)
	if err != nil || guid == (windows.GUID{}) {
		return "", errors.New("a nonzero WSL registration GUID is required")
	}
	return `Software\Microsoft\Windows\CurrentVersion\Lxss\` + guid.String(), nil
}

func readRegistration(id string) (registration, error) {
	var result registration
	keyPath, err := registrationKey(id)
	if err != nil {
		return result, err
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return result, fmt.Errorf("read exact WSL registration: %w", err)
	}
	defer key.Close()
	return readRegistrationValues(key, id)
}

func readRegistrationValues(key registry.Key, id string) (registration, error) {
	var result registration
	result.ID, _ = windows.GUIDFromString(id)
	for _, field := range []struct {
		name  string
		value *string
	}{
		{"DistributionName", &result.Name}, {"BasePath", &result.BasePath}, {"VhdFileName", &result.VHDFileName},
	} {
		value, kind, err := key.GetStringValue(field.name)
		if err != nil {
			return registration{}, fmt.Errorf("read WSL %s: %w", field.name, err)
		}
		if kind != registry.SZ {
			return registration{}, fmt.Errorf("WSL %s must be a literal string", field.name)
		}
		*field.value = value
	}
	version, kind, err := key.GetIntegerValue("Version")
	if err != nil {
		return registration{}, fmt.Errorf("read WSL version: %w", err)
	}
	if kind != registry.DWORD || version != 2 {
		return registration{}, errors.New("only WSL 2 is supported")
	}
	if _, err := result.diskPath(); err != nil {
		return registration{}, err
	}
	return result, nil
}

func (r registration) diskPath() (string, error) {
	if r.ID == (windows.GUID{}) || r.Name == "" || len(r.Name) > 256 || strings.HasPrefix(r.Name, "-") {
		return "", errors.New("invalid WSL registration identity")
	}
	for _, ch := range r.Name {
		if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.') {
			return "", errors.New("unsupported WSL distribution name")
		}
	}
	if r.VHDFileName == "" || strings.ContainsAny(r.VHDFileName, `/\:`) || filepath.Base(r.VHDFileName) != r.VHDFileName {
		return "", errors.New("WSL VHD filename must be a single component")
	}
	// Validate before Join can silently clean traversal or an empty base path.
	if !filepath.IsAbs(r.BasePath) || filepath.Clean(r.BasePath) != r.BasePath {
		return "", errors.New("invalid WSL base path")
	}
	path := r.BasePath + `\` + r.VHDFileName
	if _, err := diskPathParts(path); err != nil {
		return "", err
	}
	return path, nil
}

func (r registration) revalidate() error {
	current, err := readRegistration(r.ID.String())
	if err != nil {
		return err
	}
	if current != r {
		return errors.New("WSL registration changed; refusing to follow replacement")
	}
	return nil
}
