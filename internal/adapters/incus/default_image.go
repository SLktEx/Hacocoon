package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host/setup"
)

// EnsureStandardImage downloads the same standard Image used by ordinary
// creation. Incus owns the image cache; this is not another Hacocoon image type.
func (p *BaseProvider) EnsureStandardImage(ctx context.Context) (core.BaseName, error) {
	if err := func() (err error) {
		defer hostsetup.Track(ctx, "default_image_project")(&err)
		return p.ensureProject(ctx)
	}(); err != nil {
		return "", err
	}
	resolved, err := func() (resolved resolvedBase, err error) {
		defer hostsetup.Track(ctx, "default_image_resolve")(&err)
		return p.resolveBase(ctx, defaultBaseName)
	}()
	if err != nil {
		return "", err
	}
	// An empty target selects the configured remote, including on Incus 6,
	// which requires a second positional argument. Keep the destination project explicit.
	if err := func() (err error) {
		defer hostsetup.Track(ctx, "default_image_copy")(&err)
		result, err := p.runner.Run(ctx, "incus", "image", "copy", resolved.pinnedSource, "", "--target-project", p.project)
		if err != nil {
			return err
		}
		if result.ExitCode != 0 {
			return core.ErrRuntimeUnavailable
		}
		return nil
	}(); err != nil {
		return "", err
	}
	return defaultBaseName, nil
}
