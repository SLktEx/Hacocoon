package state

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/core"
)

func generationEpoch() (string, error) {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(nonce[:]), nil
}

// EnsureResourceGeneration never silently changes a configured compatibility
// boundary. Explicit reset creates a new epoch, including when the number is 0.
func (s *EnvironmentJSONStore) EnsureResourceGeneration(ctx context.Context, name, kind, compatibility string) (result core.ResourceGeneration, err error) {
	if !core.ValidResourceGenerationSpec(name, kind, compatibility) {
		return result, core.ErrInvalidArgument
	}
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		if current, ok := data.ResourceGenerations[name]; ok {
			if current.Kind != kind || current.Compatibility != compatibility {
				return false, core.ErrIncompatibleState
			}
			result = current
			return false, nil
		}
		epoch, err := generationEpoch()
		if err != nil {
			return false, err
		}
		result = core.ResourceGeneration{Name: name, Kind: kind, Compatibility: compatibility, Epoch: epoch}
		data.ResourceGenerations[name] = result
		return true, nil
	})
	return
}

func (s *EnvironmentJSONStore) GetResourceGeneration(ctx context.Context, name string) (result core.ResourceGeneration, err error) {
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		var ok bool
		result, ok = data.ResourceGenerations[name]
		if !ok {
			return false, core.ErrNotFound
		}
		return false, nil
	})
	return
}

func (s *EnvironmentJSONStore) ListResourceGenerations(ctx context.Context) (result []core.ResourceGeneration, err error) {
	result = []core.ResourceGeneration{}
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		for _, source := range data.ResourceGenerations {
			result = append(result, source)
		}
		return false, nil
	})
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return
}

// AdvanceResourceGeneration atomically validates both the optimistic source
// version and the candidate's durable complete ownership before adopting it.
func (s *EnvironmentJSONStore) AdvanceResourceGeneration(ctx context.Context, expected core.ResourceGeneration, candidate core.PersistentResourceRef) (result core.ResourceGeneration, err error) {
	if !core.ValidResourceGeneration(expected) || !core.ValidGenerationResource(candidate) {
		return result, core.ErrInvalidArgument
	}
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		current, ok := data.ResourceGenerations[expected.Name]
		if !ok || current != expected {
			return false, core.ErrSourceGenerationStale
		}
		if current.Number == ^uint64(0) || current.Current == candidate {
			return false, core.ErrInvalidArgument
		}
		resource, ok := data.PersistentResources[candidate.ID]
		if !ok || resource.Ref() != candidate || !resource.SourceOnly || resource.State != "ready" || resource.Kind != current.Kind {
			return false, core.ErrIncompatibleState
		}
		for _, source := range data.ResourceGenerations {
			if source.Current.ID == candidate.ID {
				return false, core.ErrStorageBusy
			}
		}
		current.Number++
		current.Current = candidate
		data.ResourceGenerations[current.Name] = current
		result = current
		return true, nil
	})
	return
}

// Reset drops only the current selection. Complete old volumes and independent
// copies survive; a new epoch fences all producers from before this reset.
func (s *EnvironmentJSONStore) ResetResourceGeneration(ctx context.Context, expected core.ResourceGeneration, compatibility string) (result core.ResourceGeneration, err error) {
	if !core.ValidResourceGeneration(expected) || !core.ValidResourceGenerationSpec(expected.Name, expected.Kind, compatibility) {
		return result, core.ErrInvalidArgument
	}
	err = s.catalogTransaction(ctx, func(data *environmentFileState) (bool, error) {
		if current, ok := data.ResourceGenerations[expected.Name]; !ok || current != expected {
			return false, core.ErrSourceGenerationStale
		}
		epoch, err := generationEpoch()
		if err != nil {
			return false, err
		}
		if epoch == expected.Epoch {
			return false, core.ErrIncompatibleState
		}
		result = expected
		result.Epoch, result.Compatibility, result.Number, result.Current = epoch, compatibility, 0, core.PersistentResourceRef{}
		data.ResourceGenerations[result.Name] = result
		return true, nil
	})
	return
}

func generationReferences(data environmentFileState, id string) bool {
	for _, source := range data.ResourceGenerations {
		if source.Current.ID == id {
			return true
		}
	}
	return false
}

func validateResourceGenerations(data environmentFileState) error {
	if len(data.ResourceGenerations) != 0 && (data.Version < 15 || data.Version > environmentStateVersion) {
		return core.ErrIncompatibleState
	}
	held := map[string]bool{}
	for name, source := range data.ResourceGenerations {
		if name != source.Name || !core.ValidResourceGeneration(source) {
			return core.ErrIncompatibleState
		}
		if source.Number == 0 {
			continue
		}
		resource, ok := data.PersistentResources[source.Current.ID]
		if !ok || resource.Ref() != source.Current || resource.State != "ready" || !resource.SourceOnly || resource.Kind != source.Kind || held[resource.ID] {
			return core.ErrIncompatibleState
		}
		held[resource.ID] = true
	}
	return nil
}

// Only a complete, exact-owned generation candidate can use this cleanup path.
// In-flight source publication and adopted sources retain their normal fences.
func (s *EnvironmentJSONStore) BeginGenerationResourceDelete(ctx context.Context, ref core.PersistentResourceRef) (core.PersistentResource, error) {
	if !core.ValidGenerationResource(ref) {
		return core.PersistentResource{}, core.ErrInvalidArgument
	}
	return s.beginPersistentResourceDelete(ctx, ref.ID, "", ref.Owner, false)
}
