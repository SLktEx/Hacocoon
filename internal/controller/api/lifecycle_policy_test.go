package controlapi

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

func (f *fakeEnvironments) ExecUser(ctx context.Context, name string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	return f.Exec(ctx, name, req)
}
func (f *fakeEnvironments) DeleteUser(ctx context.Context, name string, force bool) error {
	return f.Delete(ctx, name)
}
