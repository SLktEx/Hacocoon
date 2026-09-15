package workspace

import (
	"errors"
	"fmt"
	"os"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func validateEnvironmentName(name string) (string, error) {
	if err := core.ValidateEnvironmentName(name); err != nil {
		return "", err
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
