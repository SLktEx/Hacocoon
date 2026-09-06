package incus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/modules/standard/gitrepo"
)

// RepositoryBackend keeps custom volume mechanics and trusted-host execution
// behind the Incus boundary. It never manages a Btrfs mount or subvolume itself.
type RepositoryBackend struct {
	Runtime       *Runtime
	ProductBinary string
}

func (b *RepositoryBackend) Plan(ctx context.Context, kind, id string) (string, error) {
	if (kind != "repo" && kind != "work") || !gitrepo.ValidID(id) {
		return "", core.ErrInvalidArgument
	}
	pool, err := b.Runtime.defaultRootPool(ctx)
	if err != nil {
		return "", err
	}
	return pool + "/haco-" + kind + "-" + id, nil
}

func volumeRef(object gitrepo.Object) (string, string, error) {
	parts := strings.Split(object.NativeRef, "/")
	if len(parts) != 2 || !safeIncusRef(parts[0]) || parts[1] != "haco-"+object.Kind+"-"+object.ID || !gitrepo.ValidID(object.ID) || (object.Kind != "repo" && object.Kind != "work") || len(object.Owner) != 32 {
		return "", "", core.ErrInvalidArgument
	}
	return parts[0], parts[1], nil
}
func safeIncusRef(s string) bool {
	if s == "" || len(s) > 100 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') && c != '-' {
			return false
		}
	}
	return s[0] != '-'
}
func volumeConfig(object gitrepo.Object) map[string]string {
	return map[string]string{"user.hacocoon.owner": object.Owner, "user.hacocoon.role": object.Kind, "user.hacocoon.repository": object.Repository}
}
func (b *RepositoryBackend) CreateVolume(ctx context.Context, object gitrepo.Object, source *gitrepo.Object) error {
	pool, volume, err := volumeRef(object)
	if err != nil {
		return err
	}
	config := volumeConfig(object)
	request := map[string]any{"name": volume, "type": "custom", "content_type": "filesystem", "config": config}
	if source != nil {
		sourceConfig, err := b.inspectVolumeConfig(ctx, *source)
		if err != nil {
			return err
		}
		// Incus copies the already shifted on-disk IDs. Preserve its idmap
		// bookkeeping with the data, just as the native volume copy client
		// does; dropping it causes a second shift when the copy is mounted.
		for _, key := range []string{"volatile.idmap.last", "volatile.idmap.next"} {
			value := sourceConfig[key]
			var mapping []json.RawMessage
			if json.Unmarshal([]byte(value), &mapping) != nil || len(mapping) == 0 {
				return core.ErrIncompatibleState
			}
			config[key] = value
		}
		sourcePool, sourceVolume, err := volumeRef(*source)
		if err != nil {
			return err
		}
		if sourcePool != pool {
			return core.ErrIncompatibleState
		}
		request["source"] = map[string]any{"type": "copy", "name": sourceVolume, "pool": sourcePool, "project": b.Runtime.project, "volume_only": true}
	}
	data, err := json.Marshal(request)
	if err != nil {
		return err
	}
	_, err = b.Runtime.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project, "--data", string(data))
	return err
}
func (b *RepositoryBackend) InspectVolume(ctx context.Context, object gitrepo.Object) error {
	_, err := b.inspectVolumeConfig(ctx, object)
	return err
}

func (b *RepositoryBackend) inspectVolumeConfig(ctx context.Context, object gitrepo.Object) (map[string]string, error) {
	pool, volume, err := volumeRef(object)
	if err != nil {
		return nil, err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom/"+volume+"?project="+b.Runtime.project)
	if err != nil || result.StdoutTruncated {
		return nil, fmt.Errorf("owned volume unavailable: %w", core.ErrIncompatibleState)
	}
	var observed struct {
		Name        string            `json:"name"`
		Type        string            `json:"type"`
		ContentType string            `json:"content_type"`
		Config      map[string]string `json:"config"`
	}
	if json.Unmarshal([]byte(result.Stdout), &observed) != nil || observed.Name != volume || observed.Type != "custom" || observed.ContentType != "filesystem" {
		return nil, core.ErrIncompatibleState
	}
	for key, value := range volumeConfig(object) {
		if observed.Config[key] != value {
			return nil, fmt.Errorf("volume ownership mismatch: %w", core.ErrIncompatibleState)
		}
	}
	return observed.Config, nil
}

func (b *RepositoryBackend) Populate(ctx context.Context, object gitrepo.Object) error {
	if err := b.Runtime.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}
	if err := b.InspectVolume(ctx, object); err != nil {
		return err
	}
	pool, volume, _ := volumeRef(object)
	root := gitrepo.RepositoryRoot
	operation := "clone"
	if object.Kind == "work" {
		root = gitrepo.WorkspaceRoot
		operation = "workspace"
	}
	device := "haco-" + object.Kind + "-" + object.ID
	// A new record owns a new name; device collisions are refused by Incus.
	if _, err := b.Runtime.runner.Run(ctx, "incus", "config", "device", "add", trustedHostName, device, "disk", "pool="+pool, "source="+volume, "path="+root+"/"+object.ID, "--project", b.Runtime.project); err != nil {
		return err
	}
	_, err := b.RunGit(ctx, gitrepo.AgentRequest{Operation: operation, Repository: object.Repository, Workspace: object.ID, Remote: object.Remote, Branch: object.Branch})
	if err != nil {
		return err
	}
	if object.Kind == "work" {
		// Never leave an untrusted Workspace mounted in the trusted Host.
		if _, err := b.Runtime.runner.Run(ctx, "incus", "config", "device", "remove", trustedHostName, device, "--project", b.Runtime.project); err != nil {
			return err
		}
	}
	return nil
}

func (b *RepositoryBackend) RunGit(ctx context.Context, request gitrepo.AgentRequest) (gitrepo.Response, error) {
	if err := b.Runtime.verifyTrustedHostOwnership(ctx); err != nil {
		return gitrepo.Response{}, err
	}
	data, err := json.Marshal(request)
	if err != nil || len(data) > gitrepo.MaxMessage {
		return gitrepo.Response{}, core.ErrInvalidArgument
	}
	cmd := exec.CommandContext(ctx, "incus", "exec", trustedHostName, "--project", b.Runtime.project, "--", "/usr/local/bin/haco", "_git-agent")
	cmd.Stdin = bytes.NewReader(data)
	var output bytes.Buffer
	cmd.Stdout = &limitedGitOutput{Writer: &output, remaining: gitrepo.MaxMessage}
	cmd.Stderr = io.Discard
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		return gitrepo.Response{}, fmt.Errorf("trusted Git agent unavailable")
	}
	var response gitrepo.Response
	if json.Unmarshal(output.Bytes(), &response) != nil || len(response.Pack) > gitrepo.MaxPack {
		return gitrepo.Response{}, fmt.Errorf("invalid trusted Git response")
	}
	if response.Error != "" {
		return gitrepo.Response{}, fmt.Errorf("%s", response.Error)
	}
	return response, nil
}

type limitedGitOutput struct {
	io.Writer
	remaining int
}

func (w *limitedGitOutput) Write(data []byte) (int, error) {
	if len(data) > w.remaining {
		return 0, fmt.Errorf("Git response exceeds limit")
	}
	w.remaining -= len(data)
	return w.Writer.Write(data)
}

func (b *RepositoryBackend) ConnectGit(ctx context.Context, environment core.Environment, workspace gitrepo.Object, socket string) error {
	ref := "haco-" + environment.Name
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	if !environmentapp.MatchesRuntimeRef(environment.RuntimeRef, environmentapp.ProviderIncus, ref) {
		return core.ErrInvalidArgument
	}
	attachments, err := b.WorkspaceAttachments(ctx, workspace)
	if err != nil {
		return err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/instances/"+ref+"?project="+b.Runtime.project)
	if err != nil {
		return err
	}
	var instance struct {
		Config  map[string]string            `json:"config"`
		Devices map[string]map[string]string `json:"devices"`
	}
	if json.Unmarshal([]byte(result.Stdout), &instance) != nil || instance.Config[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue {
		return core.ErrIncompatibleState
	}
	for _, mount := range attachments {
		disk := instance.Devices[mount.Device]
		if disk["type"] != "disk" || disk["pool"] != mount.Pool || disk["source"] != mount.Volume || disk["path"] != mount.Path {
			return core.ErrIncompatibleState
		}
	}
	device := map[string]string{"type": "proxy", "bind": "instance", "listen": "unix:" + gitrepo.GuestSocket, "connect": "unix:" + socket, "mode": "0600", "uid": "0", "gid": "0"}
	if old, exists := instance.Devices["git-broker"]; exists {
		if !reflect.DeepEqual(old, device) {
			return core.ErrIncompatibleState
		}
	} else {
		args := []string{"config", "device", "add", ref, "git-broker", "proxy", "bind=instance", "listen=" + device["listen"], "connect=" + device["connect"], "mode=0600", "uid=0", "gid=0", "--project", b.Runtime.project}
		if _, err := b.Runtime.runner.Run(ctx, "incus", args...); err != nil {
			return err
		}
	}
	source, _, err := trustedClientSource(b.ProductBinary)
	if err != nil {
		return err
	}
	if _, err := b.Runtime.runner.Run(ctx, "incus", "file", "push", source, ref+"/usr/local/bin/git-remote-haco", "--project", b.Runtime.project, "--uid", "0", "--gid", "0", "--mode", "0755"); err != nil {
		return err
	}
	return nil
}

// ConfigureManagedWorkspaces is called once during controller composition.
func (r *Runtime) ConfigureManagedWorkspaces(resolve func(context.Context, string) ([]WorkspaceAttachment, error)) {
	r.managedWorkspace = resolve
}

type WorkspaceAttachment struct{ Device, Pool, Volume, Path string }

func validWorkspaceAttachment(m WorkspaceAttachment) bool {
	if !safeIncusRef(m.Pool) || !safeIncusRef(m.Volume) {
		return false
	}
	return (m.Device == "workspace" && m.Path == "/workspace") ||
		(strings.HasPrefix(m.Device, "workspace-") && gitrepo.ValidID(strings.TrimPrefix(m.Device, "workspace-")) && m.Path == "/workspace/"+strings.TrimPrefix(m.Device, "workspace-"))
}

func (b *RepositoryBackend) WorkspaceAttachments(ctx context.Context, object gitrepo.Object) ([]WorkspaceAttachment, error) {
	if object.Kind != "work" {
		return nil, core.ErrInvalidArgument
	}
	var mounts []WorkspaceAttachment
	for _, member := range object.Copies() {
		if err := b.InspectVolume(ctx, member); err != nil {
			return nil, err
		}
		pool, volume, err := volumeRef(member)
		if err != nil {
			return nil, err
		}
		mount := WorkspaceAttachment{Device: "workspace", Pool: pool, Volume: volume, Path: "/workspace"}
		if len(object.Members) != 0 {
			mount.Device += "-" + member.Repository
			mount.Path += "/" + member.Repository
		}
		if !validWorkspaceAttachment(mount) {
			return nil, core.ErrIncompatibleState
		}
		mounts = append(mounts, mount)
	}
	return mounts, nil
}
