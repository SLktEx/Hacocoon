package workspace

import (
	"context"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func (s *Service) ListSnapshots(ctx context.Context, name string) ([]core.Snapshot, error) {
	if name != "" {
		if _, err := validateEnvironmentName(name); err != nil {
			return nil, err
		}
	}
	catalog, ok := s.store.(interface {
		ListSnapshots(context.Context) ([]core.Snapshot, error)
	})
	if !ok {
		return nil, core.ErrUnsupported
	}
	saved, err := catalog.ListSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	result := []core.Snapshot{}
	for _, item := range saved {
		if name == "" || item.Source.Environment.Name == name {
			result = append(result, item)
		}
	}
	return result, nil
}
