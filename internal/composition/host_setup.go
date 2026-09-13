package composition

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
	"github.com/SLktEx/Hacocoon/internal/recipes"
	"io"
	"os"
	"path/filepath"
	"time"
)

// SetupHost is called by the Physical Host controller. The request cannot
// select source paths: client binaries are companions of that controller.
// Setup and implicit shell reconstruction share exclusion; the private recipe
// store additionally protects across controller processes and process loss.
func (a *App) SetupHost(ctx context.Context, update recipes.Update) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	if err := update.Validate(); err != nil {
		return err
	}
	if a == nil || a.Runtime == nil {
		return fmt.Errorf("Host runtime is unavailable")
	}
	if !a.hostSetupActive.TryLock() {
		return fmt.Errorf("Host setup is busy")
	}
	defer a.hostSetupActive.Unlock()
	if update.Reapply || update.ResultOnly || update.Clear {
		return hostsetup.Step(ctx, "customization", func() error { return a.HostCustomization.Apply(ctx, update) })
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
	return hostsetup.Step(ctx, "customization", func() error { return a.HostCustomization.Apply(ctx, update) })
}

// Shell entry completes mandatory provisioning before any saved customization
// on a recreated Host. A successful incarnation skips script execution.
func (a *App) PrepareTrustedHostShellStream(ctx context.Context) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error) {
	if err := a.SetupHost(ctx, recipes.Update{}); err != nil {
		return nil, err
	}
	return a.Runtime.PrepareTrustedHostShellStream(ctx)
}
