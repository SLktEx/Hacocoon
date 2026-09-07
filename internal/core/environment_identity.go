package core

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

var environmentInstancePattern = regexp.MustCompile("^env-[a-f0-9]{32}$")

// NewEnvironmentInstanceID identifies one creation, independently of its name.
func NewEnvironmentInstanceID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	return "env-" + hex.EncodeToString(value[:]), nil
}
func ValidEnvironmentInstanceID(id string) bool { return environmentInstancePattern.MatchString(id) }
