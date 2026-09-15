package incus

import (
	"context"
	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"net/url"
	"strings"
)

// This is a read-only stopped ordinary-Env check, not the trusted Host OCI
// quiesce path. The catalog copy reservation prevents Hacocoon from restarting
// or deleting the parent until positive copy completion is recorded.
func (b *PersistentResourceBackend) verifyEnvironmentGenerationSource(ctx context.Context, source, target core.PersistentResource, observed *persistentVolumeObservation) error {
	if source.Kind != CacheResourceKind || source.SourceOnly || !core.ValidEnvironmentResourceRef(source.Ref()) || !core.ValidEnvironmentInstanceID(source.EnvironmentInstance) || !core.ValidGenerationResource(target.Ref()) || !target.SourceOnly || target.EnvironmentInstance != "" || target.WorkspaceID != "" || target.CopySource != source.Ref() || observed == nil || len(observed.UsedBy) != 1 {
		return core.ErrStorageBusy
	}
	return b.verifyStoppedEnvironmentDataConsumer(ctx, source, observed, "")
}

// Shared by whole-generation collection and in-place disposable-data clearing.
// Observe the native exclusive consumer without executing any guest program.
func (b *PersistentResourceBackend) verifyStoppedEnvironmentDataConsumer(ctx context.Context, source core.PersistentResource, observed *persistentVolumeObservation, expectedRuntime string) error {
	if source.Kind != CacheResourceKind || source.SourceOnly || !core.ValidEnvironmentResourceRef(source.Ref()) || !core.ValidEnvironmentInstanceID(source.EnvironmentInstance) || observed == nil || len(observed.UsedBy) != 1 {
		return core.ErrStorageBusy
	}
	u, err := url.Parse(observed.UsedBy[0])
	if err != nil {
		return core.ErrIncompatibleState
	}
	ref := strings.TrimPrefix(u.Path, "/1.0/instances/")
	if expectedRuntime != "" && !environmentapp.MatchesRuntimeRef(expectedRuntime, environmentapp.ProviderIncus, ref) {
		return core.ErrCapabilityStale
	}
	if !environmentDataUsedBy(observed.UsedBy[0], b.Runtime.project, ref) {
		return core.ErrIncompatibleState
	}
	if err = b.Runtime.VerifyEnvironmentIdentity(ctx, ref, source.EnvironmentInstance); err != nil {
		return err
	}
	config, err := b.Runtime.environmentDataConfiguration(ctx, ref)
	if err != nil {
		return err
	}
	if config.Config["boot.autostart"] != "false" || config.Config[environmentInstanceKey] != source.EnvironmentInstance || config.Config[environmentDataKey] == "" {
		return core.ErrIncompatibleState
	}
	pool, name, err := managedResourceVolume(source)
	if err != nil {
		return err
	}
	count := 0
	for key, device := range config.Devices {
		if device["type"] == "disk" && device["pool"] == pool && device["source"] == name {
			if !strings.HasPrefix(key, environmentDataDevicePrefix) || !validEnvironmentDataTarget(device["path"]) {
				return core.ErrIncompatibleState
			}
			count++
		}
	}
	if count != 1 {
		return core.ErrIncompatibleState
	}
	status, err := b.Runtime.InspectEnvironment(ctx, ref)
	if err != nil {
		return err
	}
	if status.Absent || status.State != core.EnvironmentStopped {
		return core.ErrStorageBusy
	}
	return nil
}
