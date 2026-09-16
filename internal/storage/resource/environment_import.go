package persistentresource

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type environmentImportStore interface {
	BeginEnvironmentResourceImport(context.Context, core.WorkspaceLease, core.PersistentResourceRef) (core.PersistentResource, error)
	EnsureResourceGeneration(context.Context, string, string, string) (core.ResourceGeneration, error)
}
type environmentImportBackend interface {
	Import(context.Context, core.PersistentResource, io.ReadSeeker) error
}

func (s *Service) PlanImportedEnvironmentResources(ctx context.Context, request core.EnvironmentResourceRequest, inputs []core.EnvironmentResourceImport) ([]core.EnvironmentResourcePlan, error) {
	catalog, ok := s.Store.(environmentImportStore)
	if !ok {
		return nil, core.ErrUnsupported
	}
	if _, ok := s.Backend.(environmentImportBackend); !ok {
		return nil, core.ErrUnsupported
	}
	if len(inputs) == 0 || len(inputs) > core.MaxEnvironmentAttachments {
		return nil, core.ErrInvalidArgument
	}
	previous := ""
	for _, input := range inputs {
		if input.Archive == nil || input.Key <= previous || !core.ValidResourceGenerationSpec(input.Key, input.Kind, input.Digest) {
			return nil, core.ErrInvalidArgument
		}
		previous = input.Key
	}
	selections := make([]core.EnvironmentResourceSelection, 0, len(inputs))
	for _, input := range inputs {
		var nonce [16]byte
		_, _ = rand.Read(nonce[:])
		origin, err := catalog.EnsureResourceGeneration(ctx, "import-"+hex.EncodeToString(nonce[:]), input.Kind, input.Digest)
		if err != nil {
			return nil, err
		}
		selections = append(selections, core.EnvironmentResourceSelection{Key: input.Key, Target: input.Target, Origin: origin})
	}
	plans, err := s.PlanEnvironmentResources(ctx, request, selections)
	if err != nil {
		return nil, err
	}
	for i := range plans {
		plans[i].Resource.ImportPending = true
	}
	return plans, nil
}

func (s *Service) ImportEnvironmentResources(ctx context.Context, lease core.WorkspaceLease, inputs []core.EnvironmentResourceImport) ([]core.EnvironmentRuntimeAttachment, error) {
	if _, ok := s.Store.(environmentImportStore); !ok {
		return nil, core.ErrUnsupported
	}
	if _, ok := s.Backend.(environmentImportBackend); !ok {
		return nil, core.ErrUnsupported
	}
	if len(inputs) == 0 || len(inputs) != len(lease.Attachments) {
		return nil, core.ErrInvalidArgument
	}
	byKey := make(map[string]core.EnvironmentResourceImport, len(inputs))
	for i, input := range inputs {
		a := lease.Attachments[i]
		if input.Archive == nil || input.Key != a.Key || input.Target != a.Target || input.Kind != a.Origin.Kind || input.Digest != a.Origin.Compatibility || a.Origin.Current != (core.PersistentResourceRef{}) {
			return nil, core.ErrInvalidArgument
		}
		byKey[input.Key] = input
	}
	return s.materializeEnvironmentResources(ctx, lease, byKey)
}

func (s *Service) importReservedEnvironmentResource(ctx context.Context, r core.PersistentResource, input core.EnvironmentResourceImport) (core.PersistentResource, error) {
	if !r.ImportPending || r.Kind != input.Kind {
		return r, core.ErrCapabilityStale
	}
	backend := s.Backend.(environmentImportBackend)
	importing := Service{Store: s.Store, Backend: importBackend{Backend: s.Backend, create: func(ctx context.Context, target core.PersistentResource) error {
		return backend.Import(ctx, target, input.Archive)
	}}}
	return importing.createReserved(ctx, r, nil)
}
