package state

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
)

// Preferences are references, never a second resource catalog or creation journal.
func (s *EnvironmentJSONStore) DefaultImage(ctx context.Context) (image core.BaseName, err error) {
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		image = data.DefaultImage
		return false, nil
	})
	return
}

func (s *EnvironmentJSONStore) SetDefaultImage(ctx context.Context, image core.BaseName, initial bool) error {
	if image == "" {
		return core.ErrInvalidArgument
	}
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		if initial && data.DefaultImage != "" {
			return false, nil
		}
		data.DefaultImage = image
		return true, nil
	})
}

func (s *EnvironmentJSONStore) LastOpened(ctx context.Context) (name string, err error) {
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		name = data.LastOpened
		return false, nil
	})
	return
}

func (s *EnvironmentJSONStore) RememberOpened(ctx context.Context, expected core.Environment) error {
	return s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		current, ok := data.Environments[expected.Name]
		if !ok || current.RuntimeRef != expected.RuntimeRef || current.Workspace != expected.Workspace || !current.CreatedAt.Equal(expected.CreatedAt) {
			return false, core.ErrCapabilityStale
		}
		data.LastOpened = expected.Name
		return true, nil
	})
}
