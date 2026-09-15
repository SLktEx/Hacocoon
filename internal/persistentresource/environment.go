package persistentresource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type environmentResourceStore interface {
	BeginEnvironmentResourceMaterialization(context.Context, core.WorkspaceLease, core.PersistentResourceRef) (core.PersistentResource, error)
	RecordEnvironmentResourceCreated(context.Context, core.PersistentResource) (core.PersistentResource, error)
	BeginEnvironmentResourceDelete(context.Context, string, core.PersistentResourceRef) (core.PersistentResource, error)
}

// PlanEnvironmentResources selects fresh provider identities without creating
// data. The lifecycle owner must reserve the complete plan with the Env/lease
// before MaterializeEnvironmentResources may issue any provider request.
func (s *Service) PlanEnvironmentResources(ctx context.Context, request core.EnvironmentResourceRequest, selections []core.EnvironmentResourceSelection) ([]core.EnvironmentResourcePlan, error) {
	if !core.ValidEnvironmentInstanceID(request.InstanceID) || request.EnvironmentID == "" || len(selections) > core.MaxEnvironmentAttachments {
		return nil, core.ErrInvalidArgument
	}
	if _, ok := s.Store.(environmentResourceStore); !ok {
		return nil, core.ErrUnsupported
	}
	ordered := slices.Clone(selections)
	slices.SortFunc(ordered, func(a, b core.EnvironmentResourceSelection) int { return strings.Compare(a.Key, b.Key) })
	plans := make([]core.EnvironmentResourcePlan, 0, len(ordered))
	attachments := make([]core.EnvironmentAttachment, 0, len(ordered))
	for _, selection := range ordered {
		if selection.Origin.Current != (core.PersistentResourceRef{}) {
			if _, ok := s.Backend.(interface {
				Copy(context.Context, core.PersistentResource, core.PersistentResource) error
			}); !ok {
				return nil, core.ErrUnsupported
			}
		}
		var nonce [32]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return nil, err
		}
		r := core.PersistentResource{ID: "env-data:" + hex.EncodeToString(nonce[:16]), Owner: hex.EncodeToString(nonce[16:]), Kind: selection.Origin.Kind, State: "planned", EnvironmentInstance: request.InstanceID, CopySource: selection.Origin.Current, CreatedAt: time.Now().UTC()}
		a := core.EnvironmentAttachment{Key: selection.Key, Target: selection.Target, Resource: r.Ref(), Origin: selection.Origin}
		attachments = append(attachments, a)
		plans = append(plans, core.EnvironmentResourcePlan{Attachment: a, Resource: r})
	}
	if !core.ValidEnvironmentAttachments(attachments) {
		return nil, core.ErrInvalidArgument
	}
	for i := range plans {
		r := &plans[i].Resource
		native, err := s.Backend.Plan(ctx, r.Kind, r.Owner)
		if err != nil {
			return nil, err
		}
		if native == "" {
			return nil, core.ErrIncompatibleState
		}
		r.NativeRef = native
	}
	return plans, nil
}

func (s *Service) MaterializeEnvironmentResources(ctx context.Context, lease core.WorkspaceLease) ([]core.EnvironmentRuntimeAttachment, error) {
	store, ok := s.Store.(environmentResourceStore)
	if !ok {
		return nil, core.ErrUnsupported
	}
	result := make([]core.EnvironmentRuntimeAttachment, 0, len(lease.Attachments))
	for _, a := range lease.Attachments {
		r, err := store.BeginEnvironmentResourceMaterialization(ctx, lease, a.Resource)
		if err != nil {
			return nil, err
		}
		if r.RestoreSource != "" {
			r, err = s.materializeSavedEnvironmentResource(ctx, lease, a, r)
		} else if r.CopySource == (core.PersistentResourceRef{}) {
			r, err = s.createReserved(ctx, r, nil)
		} else {
			var source core.PersistentResource
			source, err = s.Store.GetPersistentResource(ctx, r.CopySource.ID)
			if err == nil && (source.Ref() != r.CopySource || !source.SourceOnly || source.State != "ready") {
				err = core.ErrIncompatibleState
			}
			if err == nil {
				r, err = s.copyReserved(ctx, source, r)
			}
		}
		if err != nil {
			return nil, fmt.Errorf("prepare environment data %s: %w: %w", a.Key, core.ErrRecoveryRequired, err)
		}
		result = append(result, core.EnvironmentRuntimeAttachment{Attachment: a, Resource: r})
	}
	return result, nil
}

// DeleteEnvironmentResources shares normal owned-resource deletion. Its catalog
// gate requires the exact parent's durable runtime-absence receipt. Unknown
// creation stays reserved; later children can still be cleaned independently.
func (s *Service) DeleteEnvironmentResources(ctx context.Context, lease core.WorkspaceLease) error {
	store, ok := s.Store.(environmentResourceStore)
	if !ok {
		return core.ErrUnsupported
	}
	var failures []error
	for _, a := range lease.Attachments {
		r, err := s.Store.GetPersistentResource(ctx, a.Resource.ID)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err == nil && (r.Ref() != a.Resource || r.EnvironmentInstance != lease.InstanceID) {
			err = core.ErrCapabilityStale
		}
		if err == nil && r.State == "creating" && r.CopyCompleted {
			_, err = s.RecoverCopy(ctx, r.Ref())
		}
		if err == nil {
			r, err = store.BeginEnvironmentResourceDelete(ctx, lease.InstanceID, a.Resource)
		}
		if err == nil {
			err = s.finishDelete(ctx, r)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("environment data %s retained: %w: %w", a.Key, core.ErrRecoveryRequired, err))
		}
	}
	return errors.Join(failures...)
}
