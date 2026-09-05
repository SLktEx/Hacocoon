package composition

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// SetupHost is called by the Physical Host controller. The request cannot
// select source paths: client binaries are companions of that controller.
func (a *App) SetupHost(ctx context.Context) error {
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("Host runtime is unavailable")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate controller executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve controller executable: %w", err)
	}
	return a.Runtime.SetupTrustedHost(ctx, filepath.Dir(executable))
}
