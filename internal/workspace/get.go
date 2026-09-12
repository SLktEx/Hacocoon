package workspace

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (s *Service) Get(ctx context.Context, name string) (core.Environment, error) {
	if _, err := validateEnvironmentName(name); err != nil {
		return core.Environment{}, err
	}
	return s.store.GetEnvironment(ctx, name)
}
