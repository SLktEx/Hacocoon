package state

import (
	"context"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var baseAssetOwnerPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func validateBaseAsset(a core.BaseAsset) error {
	if !strings.HasPrefix(a.ID, "base-") || !baseAssetOwnerPattern.MatchString(strings.TrimPrefix(a.ID, "base-")) || !baseAssetOwnerPattern.MatchString(a.Owner) {
		return core.ErrInvalidArgument
	}
	for _, v := range []string{string(a.Base.Name), string(a.Base.Revision), a.Provider, a.Scope, a.NativeRef} {
		if v == "" || len(v) > 1024 || !utf8.ValidString(v) || strings.TrimSpace(v) != v {
			return core.ErrInvalidArgument
		}
		for _, r := range v {
			if unicode.IsControl(r) {
				return core.ErrInvalidArgument
			}
		}
	}
	if a.Binding == "" || len(a.Binding) > 16384 || !utf8.ValidString(a.Binding) {
		return core.ErrInvalidArgument
	}
	if a.State != "planned" && a.State != "created" && a.State != "ready" {
		return core.ErrInvalidArgument
	}
	return nil
}

// FindBaseAsset never substitutes another revision, provider or storage scope.
func (s *EnvironmentJSONStore) FindBaseAsset(ctx context.Context, base core.BaseRef, provider, scope string) (core.BaseAsset, error) {
	var result core.BaseAsset
	err := s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		for _, a := range d.BaseAssets {
			if a.Base == base && a.Provider == provider && a.Scope == scope {
				result = a
				return false, nil
			}
		}
		return false, core.ErrNotFound
	})
	return result, err
}

func (s *EnvironmentJSONStore) BeginBaseAsset(ctx context.Context, a core.BaseAsset) error {
	if validateBaseAsset(a) != nil || a.State != "planned" {
		return core.ErrInvalidArgument
	}
	return s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		for _, old := range d.BaseAssets {
			if old.ID == a.ID || (old.Provider == a.Provider && ((old.Scope == a.Scope && old.Base == a.Base) || old.NativeRef == a.NativeRef)) {
				return false, core.ErrAlreadyExists
			}
		}
		d.BaseAssets[a.ID] = a
		return true, nil
	})
}

// RecordBaseAsset uses the complete immutable plan as its CAS identity. There is
// deliberately no removal transition: referenced-asset collection must first
// gain canonical reservation and positive provider-absence semantics.
func (s *EnvironmentJSONStore) RecordBaseAsset(ctx context.Context, expected core.BaseAsset, next string) error {
	if validateBaseAsset(expected) != nil {
		return core.ErrInvalidArgument
	}
	return s.catalogTransaction(ctx, func(d *environmentFileState) (bool, error) {
		current, ok := d.BaseAssets[expected.ID]
		if !ok || current != expected {
			return false, core.ErrCapabilityStale
		}
		if !((current.State == "planned" && next == "created") || (current.State == "created" && next == "ready")) {
			return false, core.ErrIncompatibleState
		}
		current.State = next
		d.BaseAssets[current.ID] = current
		return true, nil
	})
}
