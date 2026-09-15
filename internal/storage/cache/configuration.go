package cache

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const MaxConfigurationBytes = 64 << 10

// DecodeConfiguration accepts a bounded Host document. This does not choose a
// file, grant edit authority or make Environment-provided configuration trusted.
func DecodeConfiguration(data []byte) (Configuration, error) {
	var result Configuration
	trimmed := bytes.TrimSpace(data)
	if len(data) > MaxConfigurationBytes || len(trimmed) == 0 || trimmed[0] != '{' || !utf8.Valid(data) {
		return result, core.ErrInvalidArgument
	}
	// encoding/json otherwise silently accepts duplicate keys, including aliases
	// differing only in case. Reject ambiguity before interpreting any settings.
	keys := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueValue(keys, 0); err != nil {
		return result, core.ErrInvalidArgument
	}
	if _, err := keys.Token(); err != io.EOF {
		return result, core.ErrInvalidArgument
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&result); err != nil {
		return Configuration{}, core.ErrInvalidArgument
	}
	// Validation uses the same constructor as direct trusted configuration. No
	// catalog method is called by construction or decoding.
	if _, err := validateConfiguration(result); err != nil {
		return Configuration{}, err
	}
	return result, nil
}

func uniqueValue(d *json.Decoder, depth int) error {
	if depth > 4 {
		return core.ErrInvalidArgument
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	delimiter, nested := token.(json.Delim)
	if !nested {
		return nil
	}
	if delimiter != '{' && delimiter != '[' {
		return core.ErrInvalidArgument
	}
	seen := map[string]bool{}
	for d.More() {
		if delimiter == '{' {
			token, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := token.(string)
			key = strings.ToLower(key)
			if !ok || seen[key] {
				return core.ErrInvalidArgument
			}
			seen[key] = true
		}
		if err := uniqueValue(d, depth+1); err != nil {
			return err
		}
	}
	_, err = d.Token()
	return err
}
