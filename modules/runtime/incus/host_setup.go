package incus

import (
	"context"
	"fmt"
	"path/filepath"
)

// SetupTrustedHost composes the existing owned-resource reconciler and client
// provisioners. Only the controller supplies clientDirectory; it is never RPC
// input. Validate both required companions before creating provider resources.
// The temporary hacoq migration client is not a setup dependency.
func (r *Runtime) SetupTrustedHost(ctx context.Context, clientDirectory string) error {
	paths := []string{filepath.Join(clientDirectory, "haco-host"), filepath.Join(clientDirectory, "haco")}
	for _, path := range paths {
		if _, _, err := trustedClientSource(path); err != nil {
			return fmt.Errorf("validate setup client: %w", err)
		}
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
	return r.ProvisionTrustedHostProductClient(ctx, paths[1])
}
