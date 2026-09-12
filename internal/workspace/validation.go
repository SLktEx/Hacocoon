package workspace

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var environmentNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)

func validateEnvironmentName(name string) (string, error) {
	if !environmentNamePattern.MatchString(name) {
		return "", fmt.Errorf("environment name %q must use lowercase letters, digits, and internal hyphens: %w", name, core.ErrInvalidArgument)
	}
	return name, nil
}

func normalizeAccessMode(mode core.WorkspaceAccessMode) (core.WorkspaceAccessMode, error) {
	if mode == "" {
		return core.WorkspaceReadWrite, nil
	}
	switch mode {
	case core.WorkspaceReadOnly, core.WorkspaceReadWrite:
		return mode, nil
	default:
		return "", fmt.Errorf("workspace access mode %q: %w", mode, core.ErrInvalidArgument)
	}
}

func isNotFound(err error) bool {
	return errors.Is(err, core.ErrNotFound) || os.IsNotExist(err)
}
