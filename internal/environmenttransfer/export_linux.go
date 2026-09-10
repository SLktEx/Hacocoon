//go:build linux

package environmenttransfer

import (
	"context"
	"errors"
	"io"
	"reflect"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ExportSnapshots is the existing canonical capture/read/delete boundary. Capture
// refuses a running source; a nonempty result ID on error is an owned reservation.
// Read holds source deletion locks until its callback finishes. Delete confirms
// native absence before releasing ownership. No transfer-specific catalog exists.
type ExportSnapshots interface {
	CaptureStoppedSnapshot(context.Context, string) (core.Snapshot, error)
	ReadSnapshot(context.Context, string, func(context.Context, core.Snapshot) error) error
	DeleteSnapshot(context.Context, string) error
}

// Archive contains read-only native bytes, never imported management authority.
// The Incus NativeArchive implements this without exposing an SDK type or path.
type Archive interface {
	Size() int64
	Digest() string
	Reader() io.Reader
	Close() error
}

type Exporter struct {
	Snapshots ExportSnapshots
	// Component must use the protected component and verify native ownership.
	// Root and the byte budget are controller configuration, never guest input.
	Component func(context.Context, core.SnapshotComponent, string, int64) (Archive, error)
	// Workspaces reads protected catalog bindings, never guest Git configuration.
	// A nil callback is the legacy version-1 producer used by existing callers.
	Workspaces func(context.Context, core.Snapshot) ([]Workspace, error)
	Root       string
}

type ExportResult struct {
	Bundle *Staged
	// Nonempty only when this invocation's capture could not be positively removed.
	TemporarySnapshot string
}

// ExportStopped creates one coherent native copy, exports every managed component
// while reserved, and removes only that temporary copy before returning bytes.
// The source Env is never stopped, deleted or restored by this operation.
func (e *Exporter) ExportStopped(ctx context.Context, source string, limit int64) (result ExportResult, err error) {
	if !sourceName.MatchString(source) || !validLimit(limit) || e.Snapshots == nil || e.Component == nil {
		return result, core.ErrInvalidArgument
	}
	result.Bundle, err = stageProduced(ctx, e.Root, limit, func(dst io.Writer) (err error) {
		saved, err := e.Snapshots.CaptureStoppedSnapshot(ctx, source)
		result.TemporarySnapshot = saved.ID
		if saved.ID != "" {
			defer func() {
				cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
				defer cancel()
				if cleanupErr := e.Snapshots.DeleteSnapshot(cleanup, saved.ID); cleanupErr != nil {
					err = errors.Join(err, cleanupErr, core.ErrRecoveryRequired)
				} else {
					result.TemporarySnapshot = ""
				}
			}()
		}
		if err != nil {
			return err
		}
		if saved.ID == "" {
			return core.ErrIncompatibleState
		}
		return e.Snapshots.ReadSnapshot(ctx, saved.ID, func(ctx context.Context, current core.Snapshot) (err error) {
			if current.ID != saved.ID || current.Source.Environment.Name != source || !reflect.DeepEqual(current.Source, saved.Source) {
				return core.ErrCapabilityStale
			}
			components, err := snapshotComponents(current)
			if err != nil {
				return err
			}
			var workspaces []Workspace
			if e.Workspaces != nil {
				workspaces, err = e.Workspaces(ctx, current)
				if err != nil {
					return err
				}
				count := len(components) - 1
				if components[len(components)-1].Role == "oci" {
					count--
				}
				if err := (Manifest{Version: 2, Workspaces: workspaces}).validateWorkspaces(count); err != nil {
					return err
				}
			}
			var opened []Archive
			defer func() {
				for _, a := range opened {
					err = errors.Join(err, a.Close())
				}
			}()
			remaining := limit
			archives := make([]SnapshotArchive, 0, len(components))
			for _, c := range components {
				if err := ctx.Err(); err != nil {
					return err
				}
				if remaining <= 0 {
					return ErrInvalidBundle
				}
				a, exportErr := e.Component(ctx, c, e.Root, remaining)
				if a != nil {
					opened = append(opened, a)
				}
				if exportErr != nil {
					return exportErr
				}
				if a == nil {
					return ErrInvalidBundle
				}
				size, digest, reader := a.Size(), a.Digest(), a.Reader()
				if size <= 0 || size > remaining || !digestPattern.MatchString(digest) || reader == nil {
					return ErrInvalidBundle
				}
				remaining -= size
				archives = append(archives, SnapshotArchive{Component: c, Bytes: size, SHA256: digest, Data: &contextReader{ctx, reader}})
			}
			return writeSnapshot(dst, current, archives, limit, workspaces)
		})
	})
	return result, err
}
