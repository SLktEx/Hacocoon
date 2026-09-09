package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

// DeleteWorkspaceVolume removes only the recorded unattached Incus volume and
// succeeds only after a separate native observation positively proves absence.
func (b *RepositoryBackend) DeleteWorkspaceVolume(ctx context.Context, target gitrepo.Object) error {
	if target.Kind != "work" {
		return core.ErrInvalidArgument
	}
	pool, name, err := volumeRef(target)
	if err != nil {
		return err
	}
	observe := func() (bool, error) {
		result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project+"&recursion=1")
		if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
			return false, core.ErrRuntimeUnavailable
		}
		var volumes []persistentVolumeObservation
		if json.Unmarshal([]byte(result.Stdout), &volumes) != nil || volumes == nil {
			return false, core.ErrIncompatibleState
		}
		found := false
		for _, v := range volumes {
			if v.Name != name {
				continue
			}
			if found || v.Type != "custom" || v.ContentType != "filesystem" {
				return false, core.ErrIncompatibleState
			}
			for k, want := range volumeConfig(target) {
				if v.Config[k] != want {
					return false, core.ErrCapabilityStale
				}
			}
			if len(v.UsedBy) != 0 {
				return false, core.ErrStorageBusy
			}
			found = true
		}
		return found, nil
	}
	present, err := observe()
	if err != nil || !present {
		return err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "storage", "volume", "delete", pool, name, "--project", b.Runtime.project)
	if err != nil || result.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	present, err = observe()
	if err != nil {
		return err
	}
	if present {
		return core.ErrRecoveryRequired
	}
	return nil
}
