package workspace

import (
	"context"
	"errors"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type snapshotInspector interface {
	InspectSnapshotComponent(context.Context, core.SnapshotComponent) (core.SnapshotComponentInspection, error)
}

// InspectSnapshot accepts incomplete/deleting records under the existing saved-data
// locks. Failed observations do not hide other components or change ownership.
func (s *Service) InspectSnapshot(ctx context.Context, id string) (result core.SnapshotInspection, err error) {
	err = s.withSavedSnapshot(ctx, id, func(ctx context.Context, saved core.Snapshot) error {
		result = core.SnapshotInspection{ID: saved.ID, Environment: saved.Source.Environment.Name, State: saved.State, Components: []core.SnapshotComponentInspection{}}
		backend, supported := s.runtime.(snapshotInspector)
		for _, c := range saved.Components {
			item := core.SnapshotComponentInspection{Presence: "unknown", Check: "unsupported", Backing: "uninspected"}
			var observedErr error
			if supported {
				probe, cancel := context.WithTimeout(ctx, 10*time.Second)
				item, observedErr = backend.InspectSnapshotComponent(probe, c)
				cancel()
			}
			item.Role, item.State, item.Backing = c.Role, c.State, "uninspected"
			if observedErr != nil {
				item.Check = snapshotInspectionFailure(observedErr)
				if item.Presence == "" {
					item.Presence = "unknown"
				}
			}
			if !supported || observedErr != nil {
				result.Partial = true
			}
			result.Components = append(result.Components, item)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if result.Partial {
			return core.ErrRecoveryRequired
		}
		return nil
	})
	return result, err
}

func snapshotInspectionFailure(err error) string {
	switch {
	case errors.Is(err, core.ErrUnsupported):
		return "unsupported"
	case errors.Is(err, core.ErrCapabilityStale):
		return "ownership_changed"
	case errors.Is(err, core.ErrIncompatibleState), errors.Is(err, core.ErrInvalidArgument):
		return "invalid"
	default:
		return "unavailable"
	}
}
