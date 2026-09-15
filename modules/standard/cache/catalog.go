package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type CatalogGroup struct {
	State        string         `json:"state"`
	History      History        `json:"history"`
	Environments []string       `json:"environments"`
	Result       ClearResult    `json:"result"`
	Recovery     RecoveryResult `json:"recovery"`
	Failure      string         `json:"failure,omitempty"`
}
type CatalogHistory struct {
	Groups   []CatalogGroup `json:"groups"`
	Revision string         `json:"revision"`
}
type catalogSource struct {
	Source     core.ResourceGeneration
	Candidates []core.PersistentResource
}

// Catalog history includes retained generations even when every producer Env is
// gone. Display labels are observations only; exact ownership remains private to
// the shared catalog and deletion/recovery transitions.
func (w *Workflow) catalogSnapshot(ctx context.Context) (CatalogHistory, []catalogSource, error) {
	result := CatalogHistory{Groups: []CatalogGroup{}}
	if w == nil || w.Catalog == nil {
		return result, nil, core.ErrUnsupported
	}
	catalog, ok := w.Catalog.(interface {
		ListResourceGenerations(context.Context) ([]core.ResourceGeneration, error)
		ListPersistentResources(context.Context) ([]core.PersistentResource, error)
		ListEnvironments(context.Context) ([]core.Environment, error)
	})
	if !ok {
		return result, nil, core.ErrUnsupported
	}
	sources, err := catalog.ListResourceGenerations(ctx)
	if err != nil {
		return result, nil, err
	}
	resources, err := catalog.ListPersistentResources(ctx)
	if err != nil {
		return result, nil, err
	}
	envs, err := catalog.ListEnvironments(ctx)
	if err != nil {
		return result, nil, err
	}
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	sort.Slice(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })
	selected := []catalogSource{}
	for _, source := range sources {
		if source.Kind != Kind {
			continue
		}
		area := core.EnvironmentAttachment{}
		names := []string{}
		for _, env := range envs {
			for _, a := range env.Attachments {
				if a.Origin.Name == source.Name && a.Origin.Kind == Kind && a.Origin.Compatibility == source.Compatibility {
					if area.Key == "" {
						area = a
					}
					names = append(names, env.Name)
					break
				}
			}
		}
		h, _, candidates, err := sourceHistory(core.Environment{}, area, source, resources)
		if err != nil {
			return result, nil, err
		}
		result.Groups = append(result.Groups, CatalogGroup{History: h, Environments: names})
		selected = append(selected, catalogSource{Source: source, Candidates: candidates})
	}
	data, err := json.Marshal(struct {
		Sources []catalogSource
		Groups  []CatalogGroup
	}{selected, result.Groups})
	if err != nil {
		return result, nil, err
	}
	digest := sha256.Sum256(data)
	result.Revision = hex.EncodeToString(digest[:])
	return result, selected, nil
}

func (w *Workflow) CatalogHistory(ctx context.Context) (CatalogHistory, error) {
	h, _, err := w.catalogSnapshot(ctx)
	return h, err
}

// Review is checked before the first mutation. Each group then uses the same
// optimistic source reset and positive-absence cleanup as named maintenance.
// A partial result is retained and requires fresh review, never a batch replay.
func (w *Workflow) MaintainCatalog(ctx context.Context, revision string, clear bool) (CatalogHistory, error) {
	h, sources, err := w.catalogSnapshot(ctx)
	if err != nil {
		return h, err
	}
	if revision == "" || revision != h.Revision {
		return h, core.ErrCapabilityStale
	}
	if clear && w.Cleaner == nil || !clear && w.Recoverer == nil {
		return h, core.ErrUnsupported
	}
	var failures []error
	for i := range h.Groups {
		h.Groups[i].State = "not_started"
	}
	for i, source := range sources {
		if err := ctx.Err(); err != nil {
			return h, errors.Join(append(failures, err)...)
		}
		var err error
		if clear {
			h.Groups[i].Result, err = w.clearReviewed(ctx, h.Groups[i].History, source.Source, source.Candidates)
		} else {
			h.Groups[i].Recovery, err = w.recoverReviewed(ctx, h.Groups[i].History, source.Candidates)
		}
		h.Groups[i].State = "complete"
		if err != nil {
			h.Groups[i].State = "failed"
			h.Groups[i].Failure = WorkflowError(err)
			failures = append(failures, err)
		}
	}
	return h, errors.Join(failures...)
}
