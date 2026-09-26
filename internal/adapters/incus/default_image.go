package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// EnsureStandardImage downloads the same standard Image used by ordinary
// creation. Incus owns the image cache; this is not another Hacocoon image type.
func (p *BaseProvider) EnsureStandardImage(ctx context.Context) (core.BaseName, error) {
	if err := p.ensureProject(ctx); err != nil {
		return "", err
	}
	resolved, err := p.resolveBase(ctx, defaultBaseName)
	if err != nil {
		return "", err
	}
	result, err := p.runner.Run(ctx, "incus", "image", "copy", resolved.pinnedSource, "local:", "--project", p.project)
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", core.ErrRuntimeUnavailable
	}
	return defaultBaseName, nil
}
