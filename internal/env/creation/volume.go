package creation

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	gitrepo "github.com/SLktEx/Hacocoon/internal/git"
	"strings"
	"time"
)

type Volume struct {
	Name      string         `json:"name"`
	Workspace core.Workspace `json:"workspace"`
}
type volumeWorkspaces interface {
	CreateEmptyWorkspace(context.Context, string) (gitrepo.Object, error)
	ListWorkspaces(context.Context) ([]gitrepo.Object, error)
	RestoreWorkspace(context.Context, string, core.Snapshot) (gitrepo.Object, error)
}
type volumeSnapshots interface {
	CaptureSnapshot(context.Context, string) (core.Snapshot, error)
	DeleteSnapshot(context.Context, string) error
}

func (s *Service) Volumes(ctx context.Context) ([]Volume, error) {
	works, ok := s.Workspaces.(volumeWorkspaces)
	if !ok {
		return nil, core.ErrUnsupported
	}
	objects, err := works.ListWorkspaces(ctx)
	if err != nil {
		return nil, err
	}
	result := []Volume{}
	for _, object := range objects {
		if strings.HasPrefix(object.ID, "volume-") {
			result = append(result, Volume{Name: strings.TrimPrefix(object.ID, "volume-"), Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + object.Owner), Path: "managed:" + object.ID}})
		}
	}
	return result, nil
}
func (s *Service) CreateVolume(ctx context.Context, name, source string) (result Volume, err error) {
	if core.ValidateEnvironmentName(name) != nil || len(name) > 41 {
		return result, core.ErrInvalidArgument
	}
	works, ok := s.Workspaces.(volumeWorkspaces)
	if !ok {
		return result, core.ErrUnsupported
	}
	if names, ok := s.Environments.(interface {
		LockResourceName(context.Context, string) (func(), error)
	}); ok {
		unlock, err := names.LockResourceName(ctx, name)
		if err != nil {
			return result, err
		}
		defer unlock()
	}
	if catalog, ok := s.Catalog.(interface {
		GetSnapshot(context.Context, string) (core.Snapshot, error)
	}); ok {
		if _, err := catalog.GetSnapshot(ctx, name); err == nil {
			return result, core.ErrAlreadyExists
		} else if !errors.Is(err, core.ErrNotFound) {
			return result, err
		}
	}
	var object gitrepo.Object
	if source == "" {
		object, err = works.CreateEmptyWorkspace(ctx, "volume-"+name)
	} else {
		snapshots, ok := s.Environments.(volumeSnapshots)
		if !ok {
			return result, core.ErrUnsupported
		}
		var saved core.Snapshot
		saved, err = snapshots.CaptureSnapshot(ctx, source)
		if saved.ID != "" {
			defer func() {
				cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
				defer cancel()
				err = errors.Join(err, snapshots.DeleteSnapshot(cleanup, saved.ID))
			}()
		}
		if err != nil {
			return result, err
		}
		object, err = works.RestoreWorkspace(ctx, "volume-"+name, saved)
	}
	if err != nil {
		return result, err
	}
	return Volume{Name: name, Workspace: core.Workspace{ID: core.WorkspaceID("workspace:managed:" + object.Owner), Path: "managed:" + object.ID}}, nil
}
func (s *Service) InspectVolume(ctx context.Context, name string) (Volume, error) {
	if core.ValidateEnvironmentName(name) != nil || len(name) > 41 {
		return Volume{}, core.ErrInvalidArgument
	}
	work, err := s.Workspaces.Workspace(ctx, "volume-"+name)
	return Volume{Name: name, Workspace: work}, err
}
func (s *Service) DeleteVolume(ctx context.Context, name string) error {
	volume, err := s.InspectVolume(ctx, name)
	if err != nil {
		return err
	}
	return s.Environments.DeleteManagedWorkspace(ctx, volume.Workspace)
}
