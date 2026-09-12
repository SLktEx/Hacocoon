package composition

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/recipes"
	"os"
	"path/filepath"
)

// SetupHost is called by the Physical Host controller. The request cannot
// select source paths: client binaries are companions of that controller.
func (a *App) SetupHost(ctx context.Context, update recipes.Update) error {
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
	if err := a.Runtime.SetupTrustedHost(ctx, filepath.Dir(executable)); err != nil {
		return err
	}
	return a.HostCustomization.Apply(ctx, update)
}
