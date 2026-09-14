// Package experimental owns optional client configuration, outside Core.
package experimental

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"gopkg.in/yaml.v2"
)

const MaxBytes = 2 << 20

type VSCode struct {
	Settings   map[string]any `json:"settings,omitempty" yaml:"settings,omitempty"`
	Extensions Extensions     `json:"extensions,omitempty" yaml:"extensions,omitempty"`
}
type Extensions struct {
	MinReleaseAge string      `json:"minReleaseAge,omitempty" yaml:"minReleaseAge,omitempty"`
	PreRelease    string      `json:"preRelease,omitempty" yaml:"preRelease,omitempty"`
	Install       []Extension `json:"install,omitempty" yaml:"install,omitempty"`
}
type Extension struct {
	ID         string `json:"id" yaml:"id"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"`
	PreRelease string `json:"preRelease,omitempty" yaml:"preRelease,omitempty"`
}

var extensionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,127}\.[A-Za-z0-9][A-Za-z0-9-]{0,127}$`)

func ValidID(id string) bool { return extensionID.MatchString(id) }

// ReleaseAge accepts whole days or Go durations; a day is exactly 24 hours.
func ReleaseAge(value string) (time.Duration, error) {
	if value == "" {
		value = "30d"
	}
	if strings.HasSuffix(value, "d") {
		n, err := strconv.ParseUint(strings.TrimSuffix(value, "d"), 10, 32)
		if err != nil || n > 106751 {
			return 0, fmt.Errorf("invalid minReleaseAge")
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid minReleaseAge")
	}
	return d, nil
}

func (v VSCode) Validate() error {
	if _, err := ReleaseAge(v.Extensions.MinReleaseAge); err != nil {
		return err
	}
	validPre := func(s string) bool { return s == "" || s == "allow" || s == "deny" }
	if !validPre(v.Extensions.PreRelease) {
		return fmt.Errorf("preRelease must be allow or deny")
	}
	seen := map[string]bool{}
	if len(v.Extensions.Install) > 256 {
		return fmt.Errorf("too many extensions")
	}
	for _, e := range v.Extensions.Install {
		id := strings.ToLower(e.ID)
		if !ValidID(e.ID) || seen[id] {
			return fmt.Errorf("invalid or duplicate extension id")
		}
		seen[id] = true
		if !validPre(e.PreRelease) {
			return fmt.Errorf("extension preRelease must be allow or deny")
		}
		if e.Version != "" {
			if len(e.Version) > 128 {
				return fmt.Errorf("invalid extension version")
			}
			if _, err := semver.Parse(e.Version); err != nil {
				return fmt.Errorf("version must be an exact semantic version")
			}
		}
	}
	for k := range v.Settings {
		if strings.TrimSpace(k) == "" || strings.ContainsAny(k, "\x00\r\n") {
			return fmt.Errorf("invalid setting key")
		}
	}
	if _, err := json.Marshal(v.Settings); err != nil {
		return fmt.Errorf("settings must contain JSON values")
	}
	return nil
}

// DecodeObject rejects duplicate keys, multiple documents and non-JSON YAML
// values. Error messages deliberately omit potentially secret input values.
func DecodeObject(data []byte) (map[string]any, error) {
	if len(data) == 0 || len(data) > MaxBytes {
		return nil, fmt.Errorf("invalid configuration size")
	}
	var raw any
	d := yaml.NewDecoder(bytes.NewReader(data))
	d.SetStrict(true)
	if d.Decode(&raw) != nil || d.Decode(new(any)) != io.EOF {
		return nil, fmt.Errorf("invalid YAML document")
	}
	value, err := jsonValue(raw)
	if err != nil {
		return nil, err
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configuration must be an object")
	}
	return m, nil
}

func jsonValue(value any) (any, error) {
	switch v := value.(type) {
	case map[any]any:
		m := map[string]any{}
		for k, child := range v {
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("YAML keys must be strings")
			}
			x, err := jsonValue(child)
			if err != nil {
				return nil, err
			}
			m[key] = x
		}
		return m, nil
	case []any:
		for i, child := range v {
			x, err := jsonValue(child)
			if err != nil {
				return nil, err
			}
			v[i] = x
		}
		return v, nil
	default:
		if _, err := json.Marshal(v); err != nil {
			return nil, fmt.Errorf("YAML value is not JSON compatible")
		}
		return v, nil
	}
}

func ParseVSCode(data []byte) (VSCode, error) {
	m, err := DecodeObject(data)
	if err != nil {
		return VSCode{}, err
	}
	return VSCodeFromObject(m)
}

func VSCodeFromObject(m map[string]any) (VSCode, error) {
	if m == nil {
		return VSCode{}, fmt.Errorf("experimental.vscode must be an object")
	}
	// Null is a setting value, but never a substitute for schema fields.
	for k, value := range m {
		if value == nil {
			return VSCode{}, fmt.Errorf("null VS Code field")
		}
		if k == "extensions" {
			x, ok := value.(map[string]any)
			if !ok {
				return VSCode{}, fmt.Errorf("extensions must be an object")
			}
			for _, child := range x {
				if child == nil {
					return VSCode{}, fmt.Errorf("null extensions field")
				}
			}
			if list, ok := x["install"].([]any); ok {
				for _, item := range list {
					e, ok := item.(map[string]any)
					if !ok {
						return VSCode{}, fmt.Errorf("install entries must be objects")
					}
					for _, field := range e {
						if field == nil {
							return VSCode{}, fmt.Errorf("null extension field")
						}
					}
				}
			}
		}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return VSCode{}, fmt.Errorf("invalid configuration")
	}
	var v VSCode
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&v) != nil {
		return VSCode{}, fmt.Errorf("invalid experimental.vscode schema")
	}
	return v, v.Validate()
}
