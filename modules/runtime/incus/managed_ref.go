package incus

import (
	"fmt"
	"regexp"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var managedInstanceRefPattern = regexp.MustCompile(`^haco-[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)

func validateManagedInstanceRef(ref string) error {
	if !managedInstanceRefPattern.MatchString(ref) {
		return fmt.Errorf("Incus instance ref %q is not Hacocoon-managed: %w", ref, core.ErrInvalidArgument)
	}
	return nil
}

// ManagedEnvironmentRef returns the provider-local Incus identity for a
// logical Environment name. Persisted Environment state can wrap this value in
// provider routing metadata, so callers that already know they are targeting
// the local Incus provider should derive the canonical ref here instead of
// decoding persistence internals.
func ManagedEnvironmentRef(environment string) (string, error) {
	if err := validateWorkloadToken("environment", environment); err != nil {
		return "", err
	}
	ref := "haco-" + environment
	if err := validateManagedInstanceRef(ref); err != nil {
		return "", err
	}
	return ref, nil
}
