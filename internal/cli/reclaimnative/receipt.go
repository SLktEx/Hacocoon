package reclaimnative

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

const receiptLimit = 1024

func validGUID(id string) bool {
	if len(id) != 38 || id[0] != '{' || id[37] != '}' {
		return false
	}
	nonzero := false
	for i := 1; i < 37; i++ {
		c := id[i]
		if i == 9 || i == 14 || i == 19 || i == 24 {
			if c != '-' {
				return false
			}
			continue
		}
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
		if c != '0' {
			nonzero = true
		}
	}
	return nonzero
}

// These small helper/owner objects have no extensible metadata. Token decoding
// rejects duplicate (including escaped aliases), unknown and differently-cased
// field names.
func exactObject(raw []byte, fields ...string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 || len(raw) > receiptLimit {
		return nil, errors.New("invalid receipt size")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	t, err := d.Token()
	if err != nil || t != json.Delim('{') {
		return nil, errors.New("object required")
	}
	values := make(map[string]json.RawMessage, len(fields))
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		name, ok := key.(string)
		if !ok {
			return nil, errors.New("invalid field")
		}
		allowed := false
		for _, field := range fields {
			if field == name {
				allowed = true
				break
			}
		}
		if !allowed || values[name] != nil {
			return nil, errors.New("duplicate or unknown field")
		}
		var value json.RawMessage
		if err = d.Decode(&value); err != nil {
			return nil, err
		}
		values[name] = value
	}
	if t, err = d.Token(); err != nil || t != json.Delim('}') || len(values) != len(fields) {
		return nil, errors.New("incomplete object")
	}
	if _, err = d.Token(); err != io.EOF {
		return nil, errors.New("trailing receipt")
	}
	return values, nil
}

func decodePrepared(raw []byte) (string, error) {
	fields, err := exactObject(raw, "operation", "state")
	if err != nil {
		return "", err
	}
	var operation, state string
	if json.Unmarshal(fields["operation"], &operation) != nil || json.Unmarshal(fields["state"], &state) != nil || !validGUID(operation) || state != "pending" {
		return "", errors.New("invalid prepared receipt")
	}
	return operation, nil
}

// _launch's existing fixed output is dispatch-only. Require its entire one-line
// frame and a bounded nonzero Windows PID, never localized/arbitrary output.
func decodeLaunch(raw []byte) (uint32, error) {
	const prefix = "Dispatched Windows worker "
	const suffix = "; inspect the prepared operation for completion.\n"
	if len(raw) == 0 || len(raw) > receiptLimit || !bytes.HasPrefix(raw, []byte(prefix)) || !bytes.HasSuffix(raw, []byte(suffix)) {
		return 0, errors.New("invalid dispatch receipt")
	}
	pid := string(raw[len(prefix) : len(raw)-len(suffix)])
	value, err := strconv.ParseUint(pid, 10, 32)
	if err != nil || value == 0 || strconv.FormatUint(value, 10) != pid {
		return 0, errors.New("invalid dispatch identity")
	}
	return uint32(value), nil
}

func validateOwner(raw []byte, registration string) error {
	fields, err := exactObject(raw, "registration_id", "schema_version")
	if err != nil {
		return err
	}
	var owner string
	var version int
	if json.Unmarshal(fields["registration_id"], &owner) != nil || json.Unmarshal(fields["schema_version"], &version) != nil || version != 1 || !validGUID(owner) || !strings.EqualFold(owner, registration) {
		return errors.New("installed helper ownership differs")
	}
	return nil
}
