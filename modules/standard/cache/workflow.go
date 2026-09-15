package cache

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
)

type WorkflowCatalog interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	GetResourceGeneration(context.Context, string) (core.ResourceGeneration, error)
}
type Collector interface {
	CollectEnvironmentResource(context.Context, string, string) (core.ResourceGenerationPublication, error)
}
type Workflow struct {
	Settings  Settings
	Catalog   WorkflowCatalog
	Collector Collector
}

type AreaStatus struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Origin  uint64 `json:"origin"`
	Current uint64 `json:"current"`
	Stale   bool   `json:"stale"`
	State   string `json:"state"`
}

func (w *Workflow) Status(ctx context.Context, name string) ([]AreaStatus, error) {
	if w == nil || w.Catalog == nil {
		return nil, core.ErrUnsupported
	}
	env, err := w.Catalog.GetEnvironment(ctx, name)
	if err != nil {
		return nil, err
	}
	result := make([]AreaStatus, 0, len(env.Attachments))
	for _, a := range env.Attachments {
		if a.Origin.Kind != Kind {
			continue
		}
		current, err := w.Catalog.GetResourceGeneration(ctx, a.Origin.Name)
		if err != nil {
			return nil, err
		}
		result = append(result, AreaStatus{Name: a.Key, Path: a.Target, Origin: a.Origin.Number, Current: current.Number, Stale: current != a.Origin, State: "enrolled"})
	}
	return result, nil
}

// Collect resolves only Host-enrolled names, never caller paths or native IDs.
// Each result is independent; uncertainty stops before starting another copy.
func (w *Workflow) Collect(ctx context.Context, name, key string) ([]AreaStatus, error) {
	if w == nil || w.Collector == nil {
		return nil, core.ErrUnsupported
	}
	areas, err := w.Status(ctx, name)
	if err != nil {
		return nil, err
	}
	result := []AreaStatus{}
	for _, a := range areas {
		if key != "" && key != a.Name {
			continue
		}
		published, err := w.Collector.CollectEnvironmentResource(ctx, name, a.Name)
		a.State = published.State
		if a.State == "" {
			a.State = "failed"
		}
		if published.Generation.Name != "" {
			a.Current = published.Generation.Number
		}
		a.Stale = a.State == "skipped"
		result = append(result, a)
		if err != nil {
			return result, err
		}
	}
	if len(result) == 0 {
		return result, core.ErrNotFound
	}
	return result, nil
}

func WorkflowError(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, core.ErrNotFound):
		return "not_found"
	case errors.Is(err, core.ErrRecoveryRequired):
		return "recovery_required"
	case errors.Is(err, core.ErrStorageBusy):
		return "busy"
	case errors.Is(err, core.ErrIncompatibleState):
		return "incompatible_state"
	case errors.Is(err, core.ErrCapabilityStale):
		return "stale"
	case errors.Is(err, core.ErrInvalidArgument):
		return "invalid_argument"
	default:
		return "failed"
	}
}
