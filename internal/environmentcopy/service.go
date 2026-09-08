// Package environmentcopy composes owned Incus COW capture and canonical creation.
// It has no storage engine or independent lifecycle/recovery catalog.
package environmentcopy

import (
	"context"
	"errors"
	"regexp"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/snapshotrestore"
)

type Catalog interface {
	GetEnvironment(context.Context, string) (core.Environment, error)
	GetWorkspaceLease(context.Context, string) (core.WorkspaceLease, error)
}
type Snapshots interface {
	CaptureStoppedSnapshot(context.Context, string) (core.Snapshot, error)
	DeleteSnapshot(context.Context, string) error
}
type Restorer interface {
	RestoreSnapshot(context.Context, string, string) (snapshotrestore.Result, error)
}
type Service struct {
	Catalog   Catalog
	Snapshots Snapshots
	Restorer  Restorer
}
type Result struct {
	snapshotrestore.Result
	TemporarySnapshot string `json:"temporary_snapshot,omitempty"`
}

var namePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,55}[a-z0-9])?$`)

func (s *Service) CopyEnvironment(ctx context.Context, source, target string) (result Result, err error) {
	if !namePattern.MatchString(source) {
		return result, core.ErrInvalidArgument
	}
	if target == "" {
		prefix := source
		if len(prefix) > 52 {
			prefix = prefix[:52]
		}
		target = prefix + "-copy"
	}
	if !namePattern.MatchString(target) || source == target {
		return result, core.ErrInvalidArgument
	}
	// This is only an early refusal; canonical creation repeats the atomic check.
	if _, e := s.Catalog.GetEnvironment(ctx, target); e == nil {
		return result, core.ErrAlreadyExists
	} else if !errors.Is(e, core.ErrNotFound) {
		return result, e
	}
	if _, e := s.Catalog.GetWorkspaceLease(ctx, target); e == nil {
		return result, core.ErrRecoveryRequired
	} else if !errors.Is(e, core.ErrNotFound) {
		return result, e
	}
	result.Result = snapshotrestore.Result{Environment: target, State: "failed"}
	saved, err := s.Snapshots.CaptureStoppedSnapshot(ctx, source)
	result.TemporarySnapshot = saved.ID
	if saved.ID != "" {
		defer func() {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			cleanupErr := s.Snapshots.DeleteSnapshot(cleanup, saved.ID)
			if cleanupErr == nil {
				result.TemporarySnapshot = ""
			} else {
				err = errors.Join(err, cleanupErr, core.ErrRecoveryRequired)
			}
		}()
	}
	if err != nil {
		return result, err
	}
	result.Result, err = s.Restorer.RestoreSnapshot(ctx, saved.ID, target)
	if result.Environment == "" {
		result.Environment = target
		result.State = "failed"
	}
	return result, err
}
