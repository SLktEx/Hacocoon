package environment

import "context"

func (r *Router) MigrateSSHAccess(ctx context.Context, rawRef string) error {
	p, ref, err := r.resolve(rawRef)
	if err != nil {
		return err
	}
	if migration, ok := p.(interface {
		MigrateSSHAccess(context.Context, string) error
	}); ok {
		return migration.MigrateSSHAccess(ctx, ref)
	}
	return nil
}
