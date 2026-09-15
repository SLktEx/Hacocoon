package incus

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

func repositoryDataTarget(target string) bool { return strings.HasPrefix(target, "/workspace/") }

// Resolve only from the leased Workspace's trusted catalog, never from devices
// returned by the instance. The digest pins native storage ownership as well as
// placement so a changed catalog or recycled Workspace cannot authorize resume.
func (r *Runtime) environmentPlacementBinding(ctx context.Context, request core.EnvironmentResourceBinding) (string, []WorkspaceAttachment, error) {
	digest, err := environmentDataBinding(request.InstanceID, request.Attachments)
	if err != nil {
		return "", nil, err
	}
	needed := false
	for _, area := range request.Attachments {
		needed = needed || repositoryDataTarget(area.Attachment.Target)
	}
	if !needed {
		return digest, nil, nil
	}
	workID, managed := strings.CutPrefix(request.WorkspacePath, "managed:")
	if request.ReadOnly || !managed || !gitrepo.ValidID(workID) || r == nil || r.managedWorkspace == nil {
		return "", nil, core.ErrUnsupported
	}
	mounts, err := r.managedWorkspace(ctx, request.WorkspacePath)
	if err != nil {
		return "", nil, err
	}
	for _, m := range mounts {
		if _, err := environmentWorkspaceObject(m); err != nil {
			return "", nil, err
		}
	}
	selected := map[string]WorkspaceAttachment{}
	for _, area := range request.Attachments {
		if !repositoryDataTarget(area.Attachment.Target) {
			continue
		}
		found := 0
		for _, m := range mounts {
			if strings.HasPrefix(area.Attachment.Target, m.Path+"/") {
				found++
				// Remote/branch are Git presentation, not storage identity. A
				// normal branch change must not invalidate cache placement.
				m.Remote, m.Branch = "", ""
				if old, exists := selected[m.Device]; exists && old != m {
					return "", nil, core.ErrIncompatibleState
				}
				selected[m.Device] = m
			}
		}
		if found != 1 {
			return "", nil, core.ErrIncompatibleState
		}
	}
	bound := make([]WorkspaceAttachment, 0, len(selected))
	for _, m := range selected {
		bound = append(bound, m)
	}
	sort.Slice(bound, func(i, j int) bool { return bound[i].Device < bound[j].Device })
	data, err := json.Marshal(struct {
		Data, Workspace string
		Mounts          []WorkspaceAttachment
	}{digest, request.WorkspacePath, bound})
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), bound, nil
}

func environmentWorkspaceObject(m WorkspaceAttachment) (gitrepo.Object, error) {
	id, managed := strings.CutPrefix(m.Volume, "haco-work-")
	owner, err := hex.DecodeString(m.Owner)
	if !managed || !validWorkspaceAttachment(m) || !gitrepo.ValidID(m.Repository) || err != nil || len(owner) != 16 {
		return gitrepo.Object{}, core.ErrIncompatibleState
	}
	object := gitrepo.Object{ID: id, Kind: "work", Owner: m.Owner, Repository: m.Repository, NativeRef: m.Pool + "/" + m.Volume}
	if _, _, err := volumeRef(object); err != nil {
		return gitrepo.Object{}, err
	}
	if m.Device != "workspace" && m.Device != "workspace-"+m.Repository {
		return gitrepo.Object{}, core.ErrIncompatibleState
	}
	return object, nil
}

func environmentWorkspaceDevice(m WorkspaceAttachment) map[string]string {
	return map[string]string{"type": "disk", "pool": m.Pool, "source": m.Volume, "path": m.Path}
}

func matchesEnvironmentWorkspaceDevice(config environmentDataConfiguration, name string, device map[string]string, mounts []WorkspaceAttachment) bool {
	for _, m := range mounts {
		if m.Device == name && maps.Equal(device, environmentWorkspaceDevice(m)) && maps.Equal(config.ExplicitDevices[name], device) {
			return true
		}
	}
	return false
}

func (p *SandboxProvider) verifyEnvironmentWorkspaceData(ctx context.Context, ref string, mounts []WorkspaceAttachment) error {
	backend := &RepositoryBackend{Runtime: p.Runtime}
	for _, m := range mounts {
		object, err := environmentWorkspaceObject(m)
		if err != nil {
			return err
		}
		volume, err := backend.observeVolume(ctx, object)
		if err != nil {
			return err
		}
		if len(volume.UsedBy) != 1 || !environmentDataUsedBy(volume.UsedBy[0], p.project, ref) {
			return core.ErrStorageBusy
		}
	}
	return nil
}
