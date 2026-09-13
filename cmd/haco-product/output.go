package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// splitJSONFlag removes the common machine-readable output switch while
// preserving the remaining command arguments for command-specific parsing.
func splitJSONFlag(args []string) ([]string, bool, error) {
	clean := make([]string, 0, len(args))
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			if jsonOutput {
				return nil, false, fmt.Errorf("--json specified more than once")
			}
			jsonOutput = true
			continue
		}
		clean = append(clean, arg)
	}
	return clean, jsonOutput, nil
}

// writeCLIResult keeps JSON an explicit opt-in. Human output is deliberately
// simple and stable: objects become key/value blocks and arrays become lists.
// Commands with richer domain-specific tables should keep using those tables.
func writeCLIResult(out io.Writer, value any, asJSON bool) error {
	if asJSON {
		encoder := json.NewEncoder(out)
		encoder.SetEscapeHTML(false)
		return encoder.Encode(value)
	}
	if value == nil {
		return nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var normalized any
	if err := decoder.Decode(&normalized); err != nil {
		return err
	}
	return writeHumanValue(out, normalized, 0)
}

func writeHumanValue(out io.Writer, value any, indent int) error {
	pad := strings.Repeat("  ", indent)
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := v[key]
			if humanScalar(child) {
				if _, err := fmt.Fprintf(out, "%s%s: %s\n", pad, displayCell(key), humanScalarText(child)); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(out, "%s%s:\n", pad, displayCell(key)); err != nil {
				return err
			}
			if err := writeHumanValue(out, child, indent+1); err != nil {
				return err
			}
		}
		return nil
	case []any:
		if len(v) == 0 {
			_, err := fmt.Fprintf(out, "%s(none)\n", pad)
			return err
		}
		for _, child := range v {
			if humanScalar(child) {
				if _, err := fmt.Fprintf(out, "%s- %s\n", pad, humanScalarText(child)); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(out, "%s-\n", pad); err != nil {
				return err
			}
			if err := writeHumanValue(out, child, indent+1); err != nil {
				return err
			}
		}
		return nil
	default:
		_, err := fmt.Fprintf(out, "%s%s\n", pad, humanScalarText(v))
		return err
	}
}

func humanScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool, json.Number, float64:
		return true
	default:
		return false
	}
}

func humanScalarText(value any) string {
	switch v := value.(type) {
	case nil:
		return "none"
	case string:
		if v == "" {
			return "(empty)"
		}
		return displayCell(v)
	case json.Number:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}
