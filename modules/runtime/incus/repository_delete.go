package incus

import (
	"context"
	"encoding/json"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

// workspaceVolumeForDeletion verifies exact native ownership and all users.
// A missing volume is positively observed from the complete native collection.
func (b *RepositoryBackend) workspaceVolumeForDeletion(ctx context.Context, target gitrepo.Object) (*persistentVolumeObservation, error) {
	if target.Kind != "work" {
		return nil, core.ErrInvalidArgument
	}
	return b.managedVolumeForDeletion(ctx, target, "")
}

func (b *RepositoryBackend) managedVolumeForDeletion(ctx context.Context, target gitrepo.Object, allowedUser string) (*persistentVolumeObservation, error) {
	if target.Kind != "work" && target.Kind != "repo" {
		return nil, core.ErrInvalidArgument
	}
	pool, name, err := volumeRef(target)
	if err != nil {
		return nil, err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project+"&recursion=1")
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var volumes []persistentVolumeObservation
	if json.Unmarshal([]byte(result.Stdout), &volumes) != nil || volumes == nil {
		return nil, core.ErrIncompatibleState
	}
	var found *persistentVolumeObservation
	for _, v := range volumes {
		if v.Name != name {
			continue
		}
		if found != nil || v.Type != "custom" || v.ContentType != "filesystem" {
			return nil, core.ErrIncompatibleState
		}
		for k, want := range volumeConfig(target) {
			if v.Config[k] != want {
				return nil, core.ErrCapabilityStale
			}
		}
		if len(v.UsedBy) != 0 && (allowedUser == "" || len(v.UsedBy) != 1 || v.UsedBy[0] != allowedUser) {
			return nil, core.ErrStorageBusy
		}
		copy := v
		found = &copy
	}
	return found, nil
}

// CheckWorkspaceVolumeDeletion runs before a registry enters deleting, and again
// immediately before native deletion. Incus deletes child snapshots/backups with
// their volume, so those saved objects must be handled explicitly first.
func (b *RepositoryBackend) CheckWorkspaceVolumeDeletion(ctx context.Context, target gitrepo.Object) error {
	if target.Kind != "work" {
		return core.ErrInvalidArgument
	}
	return b.checkManagedVolumeDeletion(ctx, target)
}
func (b *RepositoryBackend) checkManagedVolumeDeletion(ctx context.Context, target gitrepo.Object) error {
	volume, err := b.managedVolumeForDeletion(ctx, target, "")
	if err != nil || volume == nil {
		return err
	}
	pool, name, err := volumeRef(target)
	if err != nil {
		return err
	}
	return b.Runtime.checkVolumeSavedObjects(ctx, pool, name, volume.Config)
}

// DeleteWorkspaceVolume succeeds only after independent native absence checks.
func (b *RepositoryBackend) DeleteWorkspaceVolume(ctx context.Context, target gitrepo.Object) error {
	if target.Kind != "work" {
		return core.ErrInvalidArgument
	}
	return b.deleteManagedVolume(ctx, target)
}
func (b *RepositoryBackend) deleteManagedVolume(ctx context.Context, target gitrepo.Object) error {
	if err := b.checkManagedVolumeDeletion(ctx, target); err != nil {
		return err
	}
	volume, err := b.managedVolumeForDeletion(ctx, target, "")
	if err != nil || volume == nil {
		return err
	}
	pool, name, err := volumeRef(target)
	if err != nil {
		return err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "storage", "volume", "delete", pool, name, "--project", b.Runtime.project)
	if err != nil || result.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	volume, err = b.managedVolumeForDeletion(ctx, target, "")
	if err != nil {
		return err
	}
	if volume != nil {
		return core.ErrRecoveryRequired
	}
	return nil
}
