// Package creation owns source selection for persistent Environments. Both CLI
// create and open use this service and the canonical Workspace lifecycle.
package creation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Catalog interface {
	DefaultImage(context.Context) (core.BaseName, error)
	SetDefaultImage(context.Context, core.BaseName, bool) error
	LastOpened(context.Context) (string, error)
	RememberOpened(context.Context, core.Environment) error
	GetEnvironment(context.Context, string) (core.Environment, error)
	GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error)
}
type Images interface {
	InspectBase(context.Context, core.BaseName) (core.BaseInfo, error)
}
type Workspaces interface {
	PrepareEnvironmentWorkspace(context.Context, string) (core.Workspace, error)
	Workspace(context.Context, string) (core.Workspace, error)
}
type Environments interface {
	Create(context.Context, core.EnvironmentSpec) (core.Environment, error)
	List(context.Context) ([]core.Environment, error)
	StartForWorkspace(context.Context, string, core.WorkspaceID) error
	DeleteManagedWorkspace(context.Context, core.Workspace) error
	DeleteOwnedWorkspace(context.Context, core.Workspace) error
}
type Snapshots interface {
	CreateEnvironment(context.Context, string, string) (core.Environment, error)
}
type Service struct {
	Catalog      Catalog
	Images       Images
	Workspaces   Workspaces
	Environments Environments
	Snapshots    Snapshots
}
type Request struct {
	Name     string        `json:"name,omitempty"`
	Image    core.BaseName `json:"image,omitempty"`
	Snapshot string        `json:"snapshot,omitempty"`
	Volume   string        `json:"volume,omitempty"`
}

func NewName() string {
	var nonce [8]byte
	_, _ = rand.Read(nonce[:])
	adjectives := []string{"sleepy", "brave", "quiet", "gentle", "bright", "calm", "swift", "kind"}
	animals := []string{"otter", "panda", "fox", "owl", "lynx", "seal", "hare", "wren"}
	return adjectives[int(nonce[0])%len(adjectives)] + "-" + animals[int(nonce[1])%len(animals)] + "-" + hex.EncodeToString(nonce[2:5])
}

func (s *Service) DefaultImage(ctx context.Context, image core.BaseName) (core.BaseName, error) {
	if image == "" {
		return s.Catalog.DefaultImage(ctx)
	}
	if _, err := s.Images.InspectBase(ctx, image); err != nil {
		return "", err
	}
	if err := s.Catalog.SetDefaultImage(ctx, image, false); err != nil {
		return "", err
	}
	return image, nil
}

// Create never starts an Environment. Source precedence is decided here only.
func (s *Service) Create(ctx context.Context, request Request) (core.Environment, error) {
	if request.Name == "" {
		request.Name = NewName()
	}
	if err := core.ValidateEnvironmentName(request.Name); err != nil {
		return core.Environment{}, err
	}
	if request.Snapshot != "" {
		if request.Image != "" || request.Volume != "" {
			return core.Environment{}, fmt.Errorf("snapshot cannot be combined with image or volume: %w", core.ErrInvalidArgument)
		}
		return s.Snapshots.CreateEnvironment(ctx, request.Snapshot, request.Name)
	}
	image := request.Image
	if image == "" {
		var err error
		image, err = s.Catalog.DefaultImage(ctx)
		if err != nil {
			return core.Environment{}, err
		}
		if image == "" {
			return core.Environment{}, fmt.Errorf("no default image; run haco setup or haco image default IMAGE: %w", core.ErrNotFound)
		}
	}
	if _, err := s.Images.InspectBase(ctx, image); err != nil {
		return core.Environment{}, err
	}
	// Refuse collisions before creating any data. The lifecycle repeats this
	// check atomically with the exclusive Workspace lease.
	if _, err := s.Catalog.GetEnvironment(ctx, request.Name); err == nil {
		return core.Environment{}, core.ErrAlreadyExists
	} else if !errors.Is(err, core.ErrNotFound) {
		return core.Environment{}, err
	}
	if _, err := s.Catalog.GetWorkspaceLease(ctx, request.Name); err == nil {
		return core.Environment{}, core.ErrRecoveryRequired
	} else if !errors.Is(err, core.ErrNotFound) {
		return core.Environment{}, err
	}
	var work core.Workspace
	var err error
	if request.Volume != "" {
		volume, volumeErr := s.InspectVolume(ctx, request.Volume)
		work, err = volume.Workspace, volumeErr
	} else {
		work, err = s.Workspaces.PrepareEnvironmentWorkspace(ctx, "env-"+NewName())
	}
	if err != nil {
		return core.Environment{}, err
	}
	environment, err := s.Environments.Create(ctx, core.EnvironmentSpec{
		Name: request.Name, Base: image, WorkspacePath: work.Path, ExpectedWorkspace: work.ID,
		DeferStart: true, OwnedWorkspace: request.Volume == "", Volume: request.Volume,
	})
	if err != nil && request.Volume == "" {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		// This API excludes active and uncertain leases before deleting data.
		err = errors.Join(err, s.Environments.DeleteOwnedWorkspace(cleanup, work))
	}
	return environment, err
}

// OpenTarget remembers the selected completed Environment before any client
// launch. Thus editor/start failures are retried by plain open without creating
// or repairing an existing Environment.
func (s *Service) OpenTarget(ctx context.Context, name string, fresh *Request) (core.Environment, error) {
	var environment core.Environment
	var err error
	if fresh != nil {
		if name != "" {
			return environment, core.ErrInvalidArgument
		}
		environment, err = s.Create(ctx, *fresh)
	} else {
		if name == "" {
			name, err = s.Catalog.LastOpened(ctx)
			if err != nil {
				return environment, err
			}
			all, listErr := s.Environments.List(ctx)
			if listErr != nil {
				return environment, listErr
			}
			if len(all) == 0 {
				return s.OpenTarget(ctx, "", &Request{})
			}
			if name == "" {
				// Deterministic initial selection, with no interactive navigation.
				chosen := all[0]
				for _, candidate := range all[1:] {
					if candidate.CreatedAt.After(chosen.CreatedAt) {
						chosen = candidate
					}
				}
				name = chosen.Name
			}
		}
		environment, err = s.Catalog.GetEnvironment(ctx, name)
	}
	if err != nil {
		return environment, err
	}
	if err = s.Catalog.RememberOpened(ctx, environment); err != nil {
		return environment, err
	}
	err = s.Environments.StartForWorkspace(ctx, environment.Name, environment.Workspace.ID)
	return environment, err
}
