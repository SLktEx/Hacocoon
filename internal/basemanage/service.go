// Package basemanage adds review and catalog references to native Incus images.
// It does not retain a second image catalog or implement storage lifecycle.
package basemanage

import (
	"context"
	"sort"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Identity struct {
	Name          core.BaseName `json:"name"`
	Fingerprint   string        `json:"fingerprint"`
	BuildInstance string        `json:"build_instance"`
}
type Image struct {
	Identity
	Current              bool     `json:"current"`
	Aliases              []string `json:"aliases"`
	NativeUsers          []string `json:"native_users"`
	ProtectedAliases     []string `json:"protected_aliases"`
	Environments         []string `json:"environments"`
	IndependentSnapshots []string `json:"independent_snapshots"`
}

// Backend performs the catalog recheck while native create/publication is excluded.
type Backend interface {
	ListBaseImages(context.Context) ([]Image, error)
	DeleteBaseImage(context.Context, Identity, func(context.Context) error) error
}
type Catalog interface {
	ListEnvironments(context.Context) ([]core.Environment, error)
	ListSnapshots(context.Context) ([]core.Snapshot, error)
}
type Service struct {
	Backend Backend
	Catalog Catalog
}

func matches(base *core.BaseRef, id Identity) bool {
	return base != nil && string(base.Revision) == "sha256:"+id.Fingerprint
}
func (s *Service) List(ctx context.Context) ([]Image, error) {
	if s == nil || s.Backend == nil || s.Catalog == nil {
		return nil, core.ErrUnsupported
	}
	images, err := s.Backend.ListBaseImages(ctx)
	if err != nil {
		return nil, err
	}
	envs, err := s.Catalog.ListEnvironments(ctx)
	if err != nil {
		return nil, err
	}
	snapshots, err := s.Catalog.ListSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	for i := range images {
		v := &images[i]
		v.Environments = []string{}
		v.IndependentSnapshots = []string{}
		for _, e := range envs {
			if matches(e.Base, v.Identity) {
				v.Environments = append(v.Environments, e.Name)
			}
		}
		for _, snap := range snapshots {
			if matches(snap.Source.Environment.Base, v.Identity) {
				v.IndependentSnapshots = append(v.IndependentSnapshots, snap.ID)
			}
		}
		sort.Strings(v.Environments)
		sort.Strings(v.IndependentSnapshots)
	}
	return images, nil
}
func (s *Service) Delete(ctx context.Context, id Identity) error {
	if s == nil || s.Backend == nil || s.Catalog == nil {
		return core.ErrUnsupported
	}
	return s.Backend.DeleteBaseImage(ctx, id, func(ctx context.Context) error {
		envs, err := s.Catalog.ListEnvironments(ctx)
		if err != nil {
			return err
		}
		for _, e := range envs {
			if matches(e.Base, id) {
				return core.ErrStorageBusy
			}
		}
		// Snapshot BaseRef is provenance. Saved rootfs never depends on this image.
		return nil
	})
}
