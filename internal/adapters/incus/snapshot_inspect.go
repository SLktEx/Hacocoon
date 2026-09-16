package incus

import (
	"context"
	"errors"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// InspectSnapshotComponent uses GET observations and the same pure ownership
// validators as deletion. It neither authorizes cleanup nor probes raw disk paths.
func (r *Runtime) InspectSnapshotComponent(ctx context.Context, c core.SnapshotComponent) (result core.SnapshotComponentInspection, err error) {
	result = core.SnapshotComponentInspection{Presence: "unknown", Check: "unavailable", Backing: "uninspected"}
	b, err := r.decodeSnapshotComponent(c)
	if err != nil {
		return result, err
	}
	result.Project, result.Object = b.Project, c.NativeRef
	if b.Volume != nil {
		result.Pool = b.Volume.Pool
		return r.inspectSnapshotVolume(ctx, *b.Volume, result)
	}
	if b.Rootfs != nil {
		result.Pool = b.Rootfs.Pool
	} else {
		result.Pool = b.Base.Pool
	}
	instances, err := r.readSnapshotInstances(ctx)
	if err != nil {
		return result, err
	}
	name := c.NativeRef[len("instance/"):]
	count := 0
	for _, i := range instances {
		if i.Name == name {
			count++
		}
	}
	if count > 1 {
		return result, core.ErrIncompatibleState
	}
	result.Presence, result.Check = "absent", "absent"
	if count == 0 {
		return result, nil
	}
	result.Presence, result.Check = "present", "unavailable"
	if b.Rootfs != nil {
		_, err = validateSnapshotRootfsObservation(*b.Rootfs, true, instances)
	} else {
		_, err = validateBaseStorageObservation(b.Base.storageIdentity(), instances)
	}
	if err != nil {
		return result, err
	}
	result.Check = "ready"
	return result, nil
}

func (r *Runtime) inspectSnapshotVolume(ctx context.Context, p snapshotVolumePlan, result core.SnapshotComponentInspection) (core.SnapshotComponentInspection, error) {
	volumes, err := r.readSnapshotVolumes(ctx, p.Pool)
	if err != nil {
		return result, err
	}
	count := 0
	for _, v := range volumes {
		if v.Name == p.target() {
			count++
			references := len(v.UsedBy)
			result.References = &references
		}
	}
	if count > 1 {
		result.References = nil
		return result, core.ErrIncompatibleState
	}
	result.Presence, result.Check = "absent", "absent"
	if count == 0 {
		return result, nil
	}
	result.Presence, result.Check = "present", "unavailable"
	_, err = r.validateSnapshotVolumeObservation(p, true, volumes)
	if errors.Is(err, core.ErrStorageBusy) {
		result.Check = "busy"
		return result, nil
	}
	if err != nil {
		return result, err
	}
	result.Check = "ready"
	return result, nil
}
