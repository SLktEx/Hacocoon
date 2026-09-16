package core

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

var environmentInstancePattern = regexp.MustCompile("^env-[a-f0-9]{32}$")

// NewEnvironmentInstanceID identifies one creation, independently of its name.
// crypto/rand.Read fills the buffer or terminates the process; it never returns an error.
func NewEnvironmentInstanceID() string {
	var value [16]byte
	_, _ = rand.Read(value[:])
	return "env-" + hex.EncodeToString(value[:])
}
func ValidEnvironmentInstanceID(id string) bool { return environmentInstancePattern.MatchString(id) }
