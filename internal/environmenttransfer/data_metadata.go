package environmenttransfer

import (
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// Data describes guest placement, never native ownership or source authority.
type Data struct {
	Key    string `json:"key"`
	Target string `json:"target"`
	Kind   string `json:"kind"`
}

func dataRole(index int) string { return fmt.Sprintf("data-%03d", index+1) }

func (m Manifest) validateData() error {
	if len(m.Data) > core.MaxEnvironmentAttachments {
		return ErrInvalidBundle
	}
	previous := ""
	for i, d := range m.Data {
		if d.Key <= previous || d.Kind != "build-cache" || !core.ValidResourceGenerationSpec(d.Key, d.Kind, strings.Repeat("0", 64)) || !core.ValidEnvironmentDataPath(d.Target) {
			return ErrInvalidBundle
		}
		previous = d.Key
		for _, other := range m.Data[:i] {
			if other.Target == d.Target || strings.HasPrefix(other.Target, d.Target+"/") || strings.HasPrefix(d.Target, other.Target+"/") {
				return ErrInvalidBundle
			}
		}
	}
	return nil
}
