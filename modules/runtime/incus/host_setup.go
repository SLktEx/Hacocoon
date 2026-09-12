package incus

import (
	"context"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/hostsetup"
	"path/filepath"
)

// SetupTrustedHost composes the existing owned-resource reconciler and client
// provisioners. Only the controller supplies clientDirectory; it is never RPC
// input. Validate required companions before creating provider resources.
// The temporary hacoq migration client is not a setup dependency.
func (r *Runtime) SetupTrustedHost(ctx context.Context, clientDirectory string) error {
	paths := []string{filepath.Join(clientDirectory, "haco-host"), filepath.Join(clientDirectory, "haco"), filepath.Join(clientDirectory, "haco-notify")}
	if err := hostsetup.Step(ctx, "client_validation", func() error {
		for _, path := range paths {
			if _, _, err := trustedClientSource(path); err != nil {
				return fmt.Errorf("validate setup client: %w", err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.EnsureTrustedHost(ctx); err != nil {
		return fmt.Errorf("prepare owned trusted host: %w", err)
	}
	if err := r.ensureTrustedHostClientMode(ctx); err != nil {
		return err
	}
	if err := r.ProvisionTrustedHostClient(ctx, paths[0]); err != nil {
		return err
	}
	if err := r.ProvisionTrustedHostProductClient(ctx, paths[1]); err != nil {
		return err
	}
	if err := r.provisionTrustedHostCompanion(ctx, paths[2], "/usr/local/bin/haco-notify"); err != nil {
		return err
	}
	if r.trustedHostStorage != nil {
		if err := hostsetup.Step(ctx, "host_storage", func() error { return r.trustedHostStorage(ctx) }); err != nil {
			return err
		}
	}
	if r.trustedHostNotifications != nil {
		return hostsetup.Step(ctx, "notification_setup", func() error { return r.trustedHostNotifications(ctx) })
	}
	return nil
}
