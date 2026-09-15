package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type maintenanceCatalog interface {
	ListPersistentResources(context.Context) ([]core.PersistentResource, error)
	ResetResourceGeneration(context.Context, core.ResourceGeneration, string) (core.ResourceGeneration, error)
}

type GenerationCleaner interface {
	DeleteUnselectedGeneration(context.Context, core.PersistentResourceRef) error
}

type HistoryEntry struct {
	CreatedAt time.Time `json:"created_at"`
	Origin    uint64    `json:"origin"`
	State     string    `json:"state"`
}

type History struct {
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	Current  uint64         `json:"current"`
	Revision string         `json:"revision"`
	Entries  []HistoryEntry `json:"entries"`
}

type ClearResult struct {
	Reset   bool           `json:"reset"`
	Entries []HistoryEntry `json:"entries"`
}

// historySnapshot resolves the configured name from an existing Environment and
// binds review to its full identity, selection and exact owned candidates. No
// provider locator or client-supplied resource ID participates in deletion.
func (w *Workflow) historySnapshot(ctx context.Context, name, key string) (History, core.ResourceGeneration, []core.PersistentResource, error) {
	result := History{Entries: []HistoryEntry{}}
	if w == nil || w.Catalog == nil {
		return result, core.ResourceGeneration{}, nil, core.ErrUnsupported
	}
	catalog, ok := w.Catalog.(maintenanceCatalog)
	if !ok {
		return result, core.ResourceGeneration{}, nil, core.ErrUnsupported
	}
	env, err := w.Catalog.GetEnvironment(ctx, name)
	if err != nil {
		return result, core.ResourceGeneration{}, nil, err
	}
	var area core.EnvironmentAttachment
	for _, a := range env.Attachments {
		if a.Key == key && a.Origin.Kind == Kind {
			area = a
			break
		}
	}
	if area.Key == "" {
		return result, core.ResourceGeneration{}, nil, core.ErrNotFound
	}
	current, err := w.Catalog.GetResourceGeneration(ctx, area.Origin.Name)
	if err != nil {
		return result, current, nil, err
	}
	if current.Kind != Kind || current.Compatibility != area.Origin.Compatibility {
		return result, current, nil, core.ErrIncompatibleState
	}
	all, err := catalog.ListPersistentResources(ctx)
	if err != nil {
		return result, current, nil, err
	}
	return sourceHistory(env, area, current, all)
}

func sourceHistory(env core.Environment, area core.EnvironmentAttachment, current core.ResourceGeneration, all []core.PersistentResource) (History, core.ResourceGeneration, []core.PersistentResource, error) {
	result := History{Entries: []HistoryEntry{}}
	candidates := []core.PersistentResource{}
	for _, r := range all {
		if r.Kind == Kind && r.SourceOnly && r.EnvironmentInstance == "" && r.WorkspaceID == "" && r.PublicationOrigin.Name == current.Name && r.PublicationOrigin.Compatibility == current.Compatibility {
			candidates = append(candidates, r)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].CreatedAt.Equal(candidates[j].CreatedAt) {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})
	result.Name, result.Path, result.Current = area.Key, area.Target, current.Number
	for _, r := range candidates {
		state := r.State
		if r.Ref() == current.Current {
			state = "current"
		} else if r.State == "ready" {
			state = "retained"
		}
		result.Entries = append(result.Entries, HistoryEntry{CreatedAt: r.CreatedAt, Origin: r.PublicationOrigin.Number, State: state})
	}
	data, err := json.Marshal(struct {
		Environment core.Environment
		Source      core.ResourceGeneration
		Resources   []core.PersistentResource
		Area        string
	}{env, current, candidates, area.Key})
	if err != nil {
		return result, current, nil, err
	}
	digest := sha256.Sum256(data)
	result.Revision = hex.EncodeToString(digest[:])
	return result, current, candidates, nil
}

func (w *Workflow) History(ctx context.Context, name, key string) (History, error) {
	h, _, _, err := w.historySnapshot(ctx, name, key)
	return h, err
}

// Clear resets reuse and deletes only the exact source candidates just reviewed.
// Existing independent Env data, Workspace/OCI and newly created candidates are
// excluded. State/provider fences protect snapshots and unfinished copies.
func (w *Workflow) Clear(ctx context.Context, name, key, revision string) (ClearResult, error) {
	result := ClearResult{Entries: []HistoryEntry{}}
	if w == nil || w.Cleaner == nil {
		return result, core.ErrUnsupported
	}
	h, current, candidates, err := w.historySnapshot(ctx, name, key)
	if err != nil {
		return result, err
	}
	if revision == "" || revision != h.Revision {
		return result, core.ErrCapabilityStale
	}
	return w.clearReviewed(ctx, h, current, candidates)
}

func (w *Workflow) clearReviewed(ctx context.Context, h History, current core.ResourceGeneration, candidates []core.PersistentResource) (ClearResult, error) {
	result := ClearResult{Entries: []HistoryEntry{}}
	catalog, ok := w.Catalog.(maintenanceCatalog)
	if !ok {
		return result, core.ErrUnsupported
	}
	if _, err := catalog.ResetResourceGeneration(ctx, current, current.Compatibility); err != nil {
		return result, err
	}
	result.Reset = true
	var failures []error
	for i, r := range candidates {
		entry := h.Entries[i]
		if r.State != "ready" && r.State != "deleting" {
			entry.State = "recovery-required"
			failures = append(failures, core.ErrRecoveryRequired)
		} else if err := w.Cleaner.DeleteUnselectedGeneration(ctx, r.Ref()); err != nil {
			entry.State = "cleanup-required"
			failures = append(failures, err)
		} else {
			entry.State = "deleted"
		}
		result.Entries = append(result.Entries, entry)
	}
	return result, errors.Join(failures...)
}
