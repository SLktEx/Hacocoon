//go:build linux

package incus

import (
	"context"
	"encoding/json"
	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
	"io"
)

func (b *RepositoryBackend) ImportWorkspaceVolume(ctx context.Context, object gitrepo.Object, source io.ReadSeeker) error {
	pool, name, err := volumeRef(object)
	if err != nil {
		return err
	}
	if object.Kind != "work" || object.State != "creating" || object.RestoredFrom != "" || len(object.Members) != 0 || !gitrepo.ValidID(object.Repository) || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:identity", Owner: object.Owner}) || source == nil {
		return core.ErrInvalidArgument
	}
	// Any existing name is refused, including a prior same-owner attempt. The
	// durable registry remains the exact cleanup identity after a lost reply.
	out, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project+"&recursion=1")
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var volumes []persistentVolumeObservation
	if json.Unmarshal([]byte(out.Stdout), &volumes) != nil || volumes == nil {
		return core.ErrIncompatibleState
	}
	for _, v := range volumes {
		if v.Name == name {
			return core.ErrAlreadyExists
		}
	}
	return b.Runtime.importVolumeArchive(ctx, source, b.ImportRoot, b.ImportLimit, pool, name, volumeConfig(object))
}
