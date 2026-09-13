package composition

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/SLktEx/Hacocoon/internal/basebuild"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
	"github.com/SLktEx/Hacocoon/internal/recipes"
)

// SetupHost is called by the Physical Host controller. The request cannot
// select source paths: client binaries are companions of that controller.
// Setup and implicit shell reconstruction share exclusion; the private recipe
// store additionally protects across controller processes and process loss.
func (a *App) SetupHost(ctx context.Context, update recipes.Update) error {
	return a.setupHost(ctx, update, false)
}

func (a *App) setupHost(ctx context.Context, update recipes.Update, wait bool) error {
	// Official Base publication adds two bounded Base Builder runs to first setup.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	if err := update.Validate(); err != nil {
		return err
	}
	if a == nil || a.Runtime == nil || a.BaseBuild == nil || a.HostCustomization == nil {
		return fmt.Errorf("Host runtime is unavailable")
	}
	release, err := a.acquireHostSetup(ctx, wait)
	if err != nil {
		return err
	}
	defer release()
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
	if err := hostsetup.Step(ctx, "official_bases", func() error { return a.ensureOfficialBases(ctx) }); err != nil {
		return err
	}
	return hostsetup.Step(ctx, "customization", func() error { return a.HostCustomization.Apply(ctx, update) })
}

func (a *App) ensureOfficialBases(ctx context.Context) error {
	root := envOr("HACO_ROOT", "/var/lib/hacocoon")
	marker := filepath.Join(root, "state", "official-bases-"+basebuild.OfficialBuildRevision)
	if _, err := os.Stat(marker); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect official Base Builder marker: %w", err)
	}
	for _, base := range basebuild.OfficialBases() {
		if _, err := a.BaseBuild.BuildOfficial(ctx, base); err != nil {
			return fmt.Errorf("build official Base %s: %w", base, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		return fmt.Errorf("create official Base marker directory: %w", err)
	}
	temporary := marker + ".tmp"
	if err := os.WriteFile(temporary, []byte(basebuild.OfficialBuildRevision+"\n"), 0o600); err != nil {
		return fmt.Errorf("write official Base Builder marker: %w", err)
	}
	if err := os.Rename(temporary, marker); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish official Base Builder marker: %w", err)
	}
	return nil
}

// Shell preparation may wait for an earlier operation, but never takes its
// exclusion away on cancellation. Explicit setup continues to reject overlap.
func (a *App) acquireHostSetup(ctx context.Context, wait bool) (func(), error) {
	for {
		a.hostSetupActive.Lock()
		if err := ctx.Err(); err != nil {
			a.hostSetupActive.Unlock()
			return nil, err
		}
		done := a.hostSetupDone
		if done == nil {
			done = make(chan struct{})
			a.hostSetupDone = done
			a.hostSetupActive.Unlock()
			return func() {
				a.hostSetupActive.Lock()
				a.hostSetupDone = nil
				close(done)
				a.hostSetupActive.Unlock()
			}, nil
		}
		a.hostSetupActive.Unlock()
		if !wait {
			return nil, fmt.Errorf("Host setup is busy")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-done:
		}
	}
}

// Shell entry completes mandatory provisioning before any saved customization
// on a recreated Host. A successful incarnation skips script execution.
func (a *App) PrepareTrustedHostShellStream(ctx context.Context) (func(context.Context, io.Reader, io.Writer, io.Writer) error, error) {
	if err := a.setupHost(ctx, recipes.Update{}, true); err != nil {
		return nil, err
	}
	return a.Runtime.PrepareTrustedHostShellStream(ctx)
}
