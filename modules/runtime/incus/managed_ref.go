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
