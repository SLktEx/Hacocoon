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

type EnvironmentEmptier interface {
	EmptyEnvironmentResource(context.Context, string, core.EnvironmentAttachment) error
}
type EmptyScope struct {
	Environment string `json:"environment,omitempty"`
	Area        string `json:"area,omitempty"`
	All         bool   `json:"all,omitempty"`
}

func (s EmptyScope) Valid() bool {
	return s.All && s.Environment == "" && s.Area == "" || !s.All && s.Environment != "" && len(s.Environment) <= 128 && len(s.Area) <= 64
}

type EmptyArea struct {
	Environment string `json:"environment"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	State       string `json:"state"`
	SavedCopies int    `json:"saved_copies"`
	Failure     string `json:"failure,omitempty"`
}
type EmptyPreview struct {
	Revision string      `json:"revision"`
	Areas    []EmptyArea `json:"areas"`
}
type emptyTarget struct {
	Environment core.Environment
	Attachment  core.EnvironmentAttachment
	Resource    core.PersistentResource
}

func (w *Workflow) emptySnapshot(ctx context.Context, scope EmptyScope) (EmptyPreview, []emptyTarget, error) {
	preview := EmptyPreview{Areas: []EmptyArea{}}
	if !scope.Valid() {
		return preview, nil, core.ErrInvalidArgument
	}
	if w == nil || w.Catalog == nil {
		return preview, nil, core.ErrUnsupported
	}
	catalog, ok := w.Catalog.(interface {
		ListEnvironments(context.Context) ([]core.Environment, error)
		ListPersistentResources(context.Context) ([]core.PersistentResource, error)
		ListSnapshots(context.Context) ([]core.Snapshot, error)
	})
	if !ok {
		return preview, nil, core.ErrUnsupported
	}
	var envs []core.Environment
	if scope.All {
		all, err := catalog.ListEnvironments(ctx)
		if err != nil {
			return preview, nil, err
		}
		envs = all
	} else {
		env, err := w.Catalog.GetEnvironment(ctx, scope.Environment)
		if err != nil {
			return preview, nil, err
		}
		envs = []core.Environment{env}
	}
	resources, err := catalog.ListPersistentResources(ctx)
	if err != nil {
		return preview, nil, err
	}
	snapshots, err := catalog.ListSnapshots(ctx)
	if err != nil {
		return preview, nil, err
	}
	byID := map[string]core.PersistentResource{}
	for _, r := range resources {
		byID[r.ID] = r
	}
	sort.Slice(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })
	targets := []emptyTarget{}
	for _, env := range envs {
		for _, a := range env.Attachments {
			if a.Origin.Kind != Kind || scope.Area != "" && scope.Area != a.Key {
				continue
			}
			r, ok := byID[a.Resource.ID]
			if !ok || r.Ref() != a.Resource || r.Kind != Kind || r.EnvironmentInstance == "" || r.SourceOnly || r.WorkspaceID != "" {
				return preview, nil, core.ErrIncompatibleState
			}
			saved := 0
			for _, snapshot := range snapshots {
				for _, prior := range snapshot.Source.Environment.Attachments {
					if prior.Resource == a.Resource {
						saved++
						break
					}
				}
			}
			preview.Areas = append(preview.Areas, EmptyArea{Environment: env.Name, Name: a.Key, Path: a.Target, State: r.State, SavedCopies: saved})
			targets = append(targets, emptyTarget{env, a, r})
		}
	}
	if len(targets) == 0 && !scope.All {
		return preview, nil, core.ErrNotFound
	}
	data, err := json.Marshal(struct {
		Scope   EmptyScope
		Targets []emptyTarget
		Areas   []EmptyArea
	}{scope, targets, preview.Areas})
	if err != nil {
		return preview, nil, err
	}
	sum := sha256.Sum256(data)
	preview.Revision = hex.EncodeToString(sum[:])
	return preview, targets, nil
}
func (w *Workflow) PreviewEmpty(ctx context.Context, scope EmptyScope) (EmptyPreview, error) {
	p, _, err := w.emptySnapshot(ctx, scope)
	return p, err
}

// Empty compares the complete displayed scope, then passes exact immutable
// attachments to the canonical lifecycle owner. It never sweeps new resources.
func (w *Workflow) Empty(ctx context.Context, scope EmptyScope, revision string) (EmptyPreview, error) {
	p, targets, err := w.emptySnapshot(ctx, scope)
	if err != nil {
		return p, err
	}
	if revision == "" || revision != p.Revision {
		return p, core.ErrCapabilityStale
	}
	if w.Emptier == nil {
		return p, core.ErrUnsupported
	}
	for i := range p.Areas {
		p.Areas[i].State = "not_started"
	}
	var failures []error
	for i, target := range targets {
		if err := ctx.Err(); err != nil {
			return p, errors.Join(append(failures, err)...)
		}
		err := w.Emptier.EmptyEnvironmentResource(ctx, target.Environment.Name, target.Attachment)
		p.Areas[i].State = "empty"
		if err != nil {
			p.Areas[i].State = "failed"
			p.Areas[i].Failure = WorkflowError(err)
			failures = append(failures, err)
		}
	}
	return p, errors.Join(failures...)
}
