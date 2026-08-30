package egress

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func stableSocketName(environment, runtimeRef string) (string, error) {
	environment = strings.TrimSpace(environment)
	runtimeRef = strings.TrimSpace(runtimeRef)
	if environment == "" || runtimeRef == "" || strings.ContainsAny(environment+runtimeRef, "\r\n\x00") {
		return "", core.ErrInvalidArgument
	}
	sum := sha256.Sum256([]byte(environment + "\x00" + runtimeRef))
	return hex.EncodeToString(sum[:16]), nil
}
