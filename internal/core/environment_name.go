package core

import (
	"fmt"
	"regexp"
)

var environmentNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)

// ValidateEnvironmentName is shared by normal creation and named builders.
// A display/policy name is not an ownership identity or a creation permission.
func ValidateEnvironmentName(name string) error {
	if !environmentNamePattern.MatchString(name) {
		return fmt.Errorf("environment name %q must use lowercase letters, digits, and internal hyphens: %w", name, ErrInvalidArgument)
	}
	return nil
}
