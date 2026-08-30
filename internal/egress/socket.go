package egress

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func stableSocketName(environment core.Environment) (string, error) {
	name := strings.TrimSpace(environment.Name)
	runtimeRef := strings.TrimSpace(environment.RuntimeRef)
	workspaceID := strings.TrimSpace(string(environment.Workspace.ID))
	if name == "" || runtimeRef == "" || workspaceID == "" || environment.CreatedAt.IsZero() ||
		strings.ContainsAny(name+runtimeRef+workspaceID, "\r\n\x00") {
		return "", core.ErrInvalidArgument
	}
	generation := environment.CreatedAt.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(name + "\x00" + runtimeRef + "\x00" + workspaceID + "\x00" + generation))
	return hex.EncodeToString(sum[:16]), nil
}
