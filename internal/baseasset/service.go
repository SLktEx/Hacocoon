// Package baseasset coordinates independent immutable Base retention. It does
// not build Bases, attach guest storage or implement provider-specific copying.
package baseasset

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Store interface {
	FindBaseAsset(context.Context, core.BaseRef, string, string) (core.BaseAsset, error)
	BeginBaseAsset(context.Context, core.BaseAsset) error
	RecordBaseAsset(context.Context, core.BaseAsset, string) error
}

type Backend interface {
	// Plan is read-only. The caller owns all outer identity fields.
	Plan(context.Context, core.BaseRef, string, string) (nativeRef, binding string, err error)
	Create(context.Context, core.BaseAsset) error
	Verify(context.Context, core.BaseAsset) error
}

type Service struct {
	Store    Store
	Backend  Backend
	Provider string
}

// Ensure reuses only a verified ready asset. Incomplete ownership is retained
// for recovery rather than retrying a possibly successful create blindly.
func (s *Service) Ensure(ctx context.Context, base core.BaseRef, scope string) (core.BaseAsset, error) {
	if s == nil || s.Store == nil || s.Backend == nil || s.Provider == "" || base.Name == "" || base.Revision == "" || scope == "" {
		return core.BaseAsset{}, core.ErrInvalidArgument
	}
	existing, err := s.Store.FindBaseAsset(ctx, base, s.Provider, scope)
	if err == nil {
		return s.reuse(ctx, existing)
	}
	if !errors.Is(err, core.ErrNotFound) {
		return core.BaseAsset{}, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return core.BaseAsset{}, err
	}
	owner := hex.EncodeToString(nonce[:])
	a := core.BaseAsset{ID: "base-" + owner, Owner: owner, Base: base, Provider: s.Provider, Scope: scope, State: "planned"}
	a.NativeRef, a.Binding, err = s.Backend.Plan(ctx, base, scope, owner)
	if err != nil {
		return core.BaseAsset{}, err
	}
	if err := s.Store.BeginBaseAsset(ctx, a); err != nil {
		if errors.Is(err, core.ErrAlreadyExists) {
			existing, readErr := s.Store.FindBaseAsset(ctx, base, s.Provider, scope)
			if readErr == nil {
				return s.reuse(ctx, existing)
			}
		}
		return core.BaseAsset{}, err
	}
	incomplete := func(err error) (core.BaseAsset, error) {
		return a, fmt.Errorf("Base asset %s retained for recovery: %w", a.ID, errors.Join(core.ErrRecoveryRequired, err))
	}
	if err := s.Backend.Create(ctx, a); err != nil {
		return incomplete(err)
	}
	// The create receipt is the first operation after provider completion.
	if err := s.Store.RecordBaseAsset(ctx, a, "created"); err != nil {
		return incomplete(err)
	}
	a.State = "created"
	if err := s.Backend.Verify(ctx, a); err != nil {
		return incomplete(err)
	}
	if err := s.Store.RecordBaseAsset(ctx, a, "ready"); err != nil {
		return incomplete(err)
	}
	a.State = "ready"
	return a, nil
}

func (s *Service) reuse(ctx context.Context, a core.BaseAsset) (core.BaseAsset, error) {
	if a.State != "ready" {
		return a, core.ErrRecoveryRequired
	}
	if err := s.Backend.Verify(ctx, a); err != nil {
		return a, errors.Join(core.ErrRecoveryRequired, err)
	}
	return a, nil
}
