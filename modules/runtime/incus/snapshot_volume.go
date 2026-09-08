package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

// snapshotVolumePlan is an adapter-local binding. The aggregate planner must
// persist this complete binding before createSnapshotVolume; a source name alone
// is not authority to copy it. This primitive is not a complete Environment save.
type snapshotVolumePlan struct {
	Remote                                          string `json:"remote,omitempty"`
	Branch                                          string `json:"branch,omitempty"`
	Device                                          string `json:"device,omitempty"`
	Path                                            string `json:"path,omitempty"`
	Pool, Source, SourceOwner, SourceKind, SourceID string
	SourceInstance, SourceInstanceID                string
	Owner, Role                                     string
}

func (p snapshotVolumePlan) target() string { return "haco-snapshot-" + p.Owner }
func (p snapshotVolumePlan) validate() error {
	if !safeIncusRef(p.Pool) || !safeIncusRef(p.Source) || p.Source == p.target() || p.SourceOwner == p.Owner ||
		!core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:validate", Owner: p.Owner}) ||
		!core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: "oci:validate", Owner: p.SourceOwner}) ||
		!core.ValidEnvironmentInstanceID(p.SourceInstanceID) || p.SourceInstance == trustedHostName {
		return core.ErrInvalidArgument
	}
	if err := validateManagedInstanceRef(p.SourceInstance); err != nil {
		return err
	}
	if p.Device != "" || p.Path != "" {
		if p.SourceKind == "work" && !validWorkspaceAttachment(WorkspaceAttachment{Device: p.Device, Path: p.Path, Pool: p.Pool, Volume: p.Source}) {
			return core.ErrInvalidArgument
		}
		if p.SourceKind == OCIStoreKind && (p.Device != "persistent-resource" || p.Path != OCIStorePath) {
			return core.ErrInvalidArgument
		}
	}
	if p.Remote != "" || p.Branch != "" {
		if p.SourceKind != "work" || gitrepo.ValidateRemote(p.Remote) != nil || !gitrepo.ValidBranch(p.Branch) {
			return core.ErrInvalidArgument
		}
	}
	switch p.SourceKind {
	case "work":
		if !strings.HasPrefix(p.Source, "haco-work-") || !gitrepo.ValidID(strings.TrimPrefix(p.Source, "haco-work-")) || !gitrepo.ValidID(p.SourceID) || !strings.HasPrefix(p.Role, "workspace:") || !gitrepo.ValidID(strings.TrimPrefix(p.Role, "workspace:")) {
			return core.ErrInvalidArgument
		}
	case OCIStoreKind:
		if p.Source != "haco-persistent-"+p.SourceOwner || !core.ValidPersistentResourceRef(core.PersistentResourceRef{ID: p.SourceID, Owner: p.SourceOwner}) || p.Role != "oci" {
			return core.ErrInvalidArgument
		}
	default:
		return core.ErrUnsupported
	}
	return nil
}
func (p snapshotVolumePlan) targetConfig() map[string]string {
	return map[string]string{
		"user.hacocoon.owner": p.Owner, "user.hacocoon.kind": "snapshot-volume", "user.hacocoon.snapshot-role": p.Role,
		"user.hacocoon.snapshot-source": p.Pool + "/" + p.Source, "user.hacocoon.snapshot-source-owner": p.SourceOwner,
		"user.hacocoon.snapshot-instance":      p.SourceInstanceID,
		"user.hacocoon.snapshot-instance-name": p.SourceInstance, "user.hacocoon.snapshot-source-kind": p.SourceKind, "user.hacocoon.snapshot-source-id": p.SourceID,
	}
}
func (r *Runtime) snapshotVolumeObservation(ctx context.Context, p snapshotVolumePlan, target bool) (*persistentVolumeObservation, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}
	name := p.Source
	if target {
		name = p.target()
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+p.Pool+"/volumes/custom?project="+r.project+"&recursion=1")
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return nil, core.ErrRuntimeUnavailable
	}
	var volumes []persistentVolumeObservation
	if json.Unmarshal([]byte(out.Stdout), &volumes) != nil || volumes == nil {
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
		expected := p.targetConfig()
		if !target {
			expected = map[string]string{"user.hacocoon.owner": p.SourceOwner}
			if p.SourceKind == "work" {
				expected["user.hacocoon.role"] = "work"
				expected["user.hacocoon.repository"] = p.SourceID
			} else {
				expected["user.hacocoon.kind"] = OCIStoreKind
				expected["user.hacocoon.resource"] = p.SourceID
				if !matchesSourceOnlyMarker(v.Config["user.hacocoon.source-only"], false) {
					return nil, core.ErrIncompatibleState
				}
			}
		}
		for k, want := range expected {
			if v.Config[k] != want {
				return nil, core.ErrCapabilityStale
			}
		}
		for _, user := range v.UsedBy {
			if target || user != "/1.0/instances/"+p.SourceInstance+"?project="+r.project {
				return nil, core.ErrStorageBusy
			}
		}
		copy := v
		found = &copy
	}
	return found, nil
}

// createSnapshotVolume performs an independent same-pool Incus COW copy. It does
// no follow-up verification after creation, allowing the coordinator to durably
// record the created identity before another fallible provider operation.
func (r *Runtime) createSnapshotVolume(ctx context.Context, p snapshotVolumePlan) error {
	if err := p.validate(); err != nil {
		return err
	}
	if err := r.VerifyEnvironmentIdentity(ctx, p.SourceInstance, p.SourceInstanceID); err != nil {
		return err
	}
	status, err := r.InspectEnvironment(ctx, p.SourceInstance)
	if err != nil {
		return err
	}
	if status.State != core.EnvironmentStopped {
		return core.ErrIncompatibleState
	}
	out, err := r.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+p.Pool)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var pool struct{ Name, Driver string }
	if json.Unmarshal([]byte(out.Stdout), &pool) != nil || pool.Name != p.Pool || pool.Driver != "btrfs" {
		return core.ErrIncompatibleState
	}
	source, err := r.snapshotVolumeObservation(ctx, p, false)
	if err != nil {
		return err
	}
	if source == nil {
		return core.ErrNotFound
	}
	config := p.targetConfig()
	// Preserve only Incus idmap bookkeeping; never copy guest/admin config wholesale.
	for _, key := range []string{"volatile.idmap.last", "volatile.idmap.next"} {
		value := source.Config[key]
		if value == "" {
			continue
		}
		var mapping []json.RawMessage
		if json.Unmarshal([]byte(value), &mapping) != nil || mapping == nil {
			return core.ErrIncompatibleState
		}
		config[key] = value
	}
	request := map[string]any{"name": p.target(), "type": "custom", "content_type": "filesystem", "config": config,
		"source": map[string]any{"type": "copy", "name": p.Source, "pool": p.Pool, "project": r.project, "volume_only": true}}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	out, err = r.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", "/1.0/storage-pools/"+p.Pool+"/volumes/custom?project="+r.project, "--data", string(data))
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return fmt.Errorf("snapshot volume creation unconfirmed: %w", core.ErrRecoveryRequired)
	}
	return nil
}
func (r *Runtime) verifySnapshotVolume(ctx context.Context, p snapshotVolumePlan) error {
	v, err := r.snapshotVolumeObservation(ctx, p, true)
	if err != nil {
		return err
	}
	if v == nil {
		return core.ErrNotFound
	}
	return nil
}
func (r *Runtime) deleteSnapshotVolume(ctx context.Context, p snapshotVolumePlan) error {
	v, err := r.snapshotVolumeObservation(ctx, p, true)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	result, err := r.runner.Run(ctx, "incus", "storage", "volume", "delete", p.Pool, p.target(), "--project", r.project)
	if err != nil || result.ExitCode != 0 {
		return core.ErrRecoveryRequired
	}
	v, err = r.snapshotVolumeObservation(ctx, p, true)
	if err != nil {
		return err
	}
	if v != nil {
		return core.ErrRecoveryRequired
	}
	return nil
}
