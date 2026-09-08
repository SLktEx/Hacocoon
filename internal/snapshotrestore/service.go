// Package snapshotrestore connects Incus-backed data copies to canonical Env
// creation. It owns no storage engine, backup or runtime recovery state machine.
package snapshotrestore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

type Catalog interface {
	GetSnapshot(context.Context, string) (core.Snapshot, error)
	GetEnvironment(context.Context, string) (core.Environment, error)
	GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error)
}
type Environments interface {
	CreateFromSnapshot(context.Context, core.EnvironmentSpec, string) (core.Environment, error)
	StartForWorkspace(context.Context, string, core.WorkspaceID) error
	CleanupRestoredData(context.Context, core.Workspace, func(context.Context) error) error
}
type Workspaces interface {
	RestoreWorkspace(context.Context, string, core.Snapshot) (gitrepo.Object, error)
	DeleteRestoredCopy(context.Context, gitrepo.Object) error
}
type Stores interface {
	RestoreSnapshot(context.Context, string, core.Snapshot, core.WorkspaceID) (core.PersistentResource, error)
	DeleteRestoredCopy(context.Context, core.PersistentResource) error
}
type Service struct {
	Catalog      Catalog
	Environments Environments
	Workspaces   Workspaces
	Stores       Stores
}

// Result exposes public names even when cleanup/start fails, never native bindings.
type Result struct {
	Environment string `json:"environment"`
	Workspace   string `json:"workspace,omitempty"`
	OCI         string `json:"oci,omitempty"`
	State       string `json:"state"`
}

var envName = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)
var savedID = regexp.MustCompile(`^snap-[a-f0-9]{32}$`)

func (s *Service) RestoreSnapshot(ctx context.Context, id, name string) (Result, error) {
	if !savedID.MatchString(id) {
		return Result{}, core.ErrInvalidArgument
	}
	saved, err := s.Catalog.GetSnapshot(ctx, id)
	if err != nil {
		return Result{}, err
	}
	if saved.State != "ready" {
		return Result{}, core.ErrIncompatibleState
	}
	if name == "" {
		prefix := saved.Source.Environment.Name
		if len(prefix) > 48 {
			prefix = prefix[:48]
		}
		name = prefix + "-restored"
	}
	if !envName.MatchString(name) {
		return Result{}, core.ErrInvalidArgument
	}
	if _, err := s.Catalog.GetEnvironment(ctx, name); err == nil {
		return Result{}, core.ErrAlreadyExists
	} else if !errors.Is(err, core.ErrNotFound) {
		return Result{}, err
	}
	if _, err := s.Catalog.GetWorkspaceLease(ctx, name); err == nil {
		return Result{}, core.ErrRecoveryRequired
	} else if !errors.Is(err, core.ErrNotFound) {
		return Result{}, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Result{}, err
	}
	workName := "restore-" + hex.EncodeToString(nonce[:8])
	result := Result{Environment: name, State: "failed"}
	object, err := s.Workspaces.RestoreWorkspace(ctx, workName, saved)
	if object.ID != "" {
		result.Workspace = object.ID
	}
	if err != nil {
		if object.ID != "" {
			result.State = "cleanup-required"
		}
		return result, err
	}
	work := core.Workspace{ID: core.WorkspaceID("workspace:managed:" + object.Owner), Path: "managed:" + object.ID}
	var resource core.PersistentResource
	fail := func(cause error) (Result, error) {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		cleanupErr := s.Environments.CleanupRestoredData(cleanup, work, func(ctx context.Context) error {
			var failures []error
			if resource.ID != "" {
				if err := s.Stores.DeleteRestoredCopy(ctx, resource); err != nil {
					failures = append(failures, err)
				} else {
					result.OCI = ""
				}
			}
			if err := s.Workspaces.DeleteRestoredCopy(ctx, object); err != nil {
				failures = append(failures, err)
			} else {
				result.Workspace = ""
			}
			return errors.Join(failures...)
		})
		if cleanupErr != nil {
			result.State = "cleanup-required"
			return result, fmt.Errorf("restore %s failed; owned residue retained: %w", name, errors.Join(cause, cleanupErr, core.ErrRecoveryRequired))
		}
		return result, fmt.Errorf("restore %s failed; new data copies removed: %w", name, cause)
	}
	if saved.Source.Environment.PersistentResource.ID != "" {
		resource, err = s.Stores.RestoreSnapshot(ctx, "oci:"+workName, saved, work.ID)
		result.OCI = resource.ID
		if err != nil {
			return fail(err)
		}
	}
	environment, err := s.Environments.CreateFromSnapshot(ctx, core.EnvironmentSpec{Name: name, WorkspacePath: work.Path, PersistentResource: resource.ID, SkipDefaultResource: resource.ID == ""}, id)
	if err != nil {
		return fail(err)
	}
	result.State = "start-failed"
	if err := s.Environments.StartForWorkspace(ctx, environment.Name, work.ID); err != nil {
		return result, err
	}
	result.State = "running"
	return result, nil
}
