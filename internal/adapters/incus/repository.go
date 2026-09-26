package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/SLktEx/Hacocoon/internal/adapters/git"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/env"
	"github.com/SLktEx/Hacocoon/internal/git"
)

// RepositoryBackend keeps custom volume mechanics and trusted-host execution
// behind the Incus boundary. It never manages a Btrfs mount or subvolume itself.
type RepositoryBackend struct {
	Runtime       *Runtime
	ProductBinary string
	// ImportRoot and ImportLimit are trusted controller staging configuration.
	ImportRoot  string
	ImportLimit int64
}

func (b *RepositoryBackend) Plan(ctx context.Context, kind, id string) (string, error) {
	if (kind != "repo" && kind != "work") || !gitadapter.ValidID(id) {
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
	if len(parts) != 2 || !safeIncusRef(parts[0]) || parts[1] != "haco-"+object.Kind+"-"+object.ID || !gitadapter.ValidID(object.ID) || (object.Kind != "repo" && object.Kind != "work") || len(object.Owner) != 32 {
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
	result, createErr := b.Runtime.runner.Run(ctx, "incus", "query", "-X", "POST", "--wait", "/1.0/storage-pools/"+pool+"/volumes/custom?project="+b.Runtime.project, "--data", string(data))
	if createErr == nil && result.ExitCode == 0 {
		return nil
	}
	if createErr == nil {
		createErr = core.ErrRuntimeUnavailable
	}
	// A lost POST reply is not a failed create. Reconcile against the exact
	// persisted owner before deciding that the operation still needs recovery.
	reconcile, cancel := context.WithTimeout(context.WithoutCancel(ctx), b.Runtime.cleanupTimeout)
	defer cancel()
	if err := b.InspectVolume(reconcile, object); err == nil {
		return nil
	}
	return createErr
}
func (b *RepositoryBackend) InspectVolume(ctx context.Context, object gitrepo.Object) error {
	_, err := b.inspectVolumeConfig(ctx, object)
	return err
}

func (b *RepositoryBackend) inspectVolumeConfig(ctx context.Context, object gitrepo.Object) (map[string]string, error) {
	observed, err := b.observeVolume(ctx, object)
	return observed.Config, err
}

type repositoryVolumeObservation struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	ContentType string            `json:"content_type"`
	Config      map[string]string `json:"config"`
	UsedBy      []string          `json:"used_by"`
}

func (b *RepositoryBackend) observeVolume(ctx context.Context, object gitrepo.Object) (repositoryVolumeObservation, error) {
	var observed repositoryVolumeObservation
	pool, volume, err := volumeRef(object)
	if err != nil {
		return observed, err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/storage-pools/"+pool+"/volumes/custom/"+volume+"?project="+b.Runtime.project)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated {
		return observed, fmt.Errorf("owned volume unavailable: %w", core.ErrIncompatibleState)
	}
	if json.Unmarshal([]byte(result.Stdout), &observed) != nil || observed.Name != volume || observed.Type != "custom" || observed.ContentType != "filesystem" {
		return observed, core.ErrIncompatibleState
	}
	for key, value := range volumeConfig(object) {
		if observed.Config[key] != value {
			return observed, fmt.Errorf("volume ownership mismatch: %w", core.ErrIncompatibleState)
		}
	}
	return observed, nil
}

func (b *RepositoryBackend) ensureRepositoryDevice(ctx context.Context, device, pool, volume, target string) error {
	result, addErr := b.Runtime.runner.Run(ctx, "incus", "config", "device", "add", trustedHostName, device, "disk", "pool="+pool, "source="+volume, "path="+target, "--project", b.Runtime.project)
	if addErr == nil && result.ExitCode == 0 {
		return nil
	}
	if addErr == nil {
		addErr = core.ErrRuntimeUnavailable
	}
	// Retried population may find the device left by the previous attempt, or
	// may have lost the successful add reply. Only the exact expected device is
	// accepted; a same-name foreign device remains a hard failure.
	reconcile, cancel := context.WithTimeout(context.WithoutCancel(ctx), b.Runtime.cleanupTimeout)
	defer cancel()
	out, err := b.Runtime.runner.Run(reconcile, "incus", "query", "/1.0/instances/"+trustedHostName+"?project="+b.Runtime.project)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return addErr
	}
	var instance struct {
		Name    string                       `json:"name"`
		Type    string                       `json:"type"`
		Devices map[string]map[string]string `json:"devices"`
	}
	if json.Unmarshal([]byte(out.Stdout), &instance) != nil || instance.Name != trustedHostName || instance.Type != "container" {
		return addErr
	}
	expected := map[string]string{"type": "disk", "pool": pool, "source": volume, "path": target}
	if !reflect.DeepEqual(instance.Devices[device], expected) {
		return addErr
	}
	return nil
}

func (b *RepositoryBackend) Populate(ctx context.Context, object gitrepo.Object) error {
	if err := b.Runtime.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}
	if err := b.InspectVolume(ctx, object); err != nil {
		return err
	}
	pool, volume, _ := volumeRef(object)
	root := gitadapter.RepositoryRoot
	operation := "clone"
	if object.Kind == "work" {
		root = gitadapter.WorkspaceRoot
		operation = "workspace"
	}
	device := "haco-" + object.Kind + "-" + object.ID
	if err := b.ensureRepositoryDevice(ctx, device, pool, volume, root+"/"+object.ID); err != nil {
		return err
	}
	_, err := b.RunGit(ctx, gitadapter.AgentRequest{Operation: operation, Repository: object.Repository, Workspace: object.ID, Remote: object.Remote, Branch: object.Branch})
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

func (b *RepositoryBackend) RunGit(ctx context.Context, request gitadapter.AgentRequest) (gitadapter.Response, error) {
	if err := b.Runtime.verifyTrustedHostOwnership(ctx); err != nil {
		return gitadapter.Response{}, err
	}
	input, err := gitadapter.AgentRequestBody(request)
	if err != nil {
		return gitadapter.Response{}, core.ErrInvalidArgument
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, "incus", "exec", trustedHostName, "--project", b.Runtime.project, "--", "/usr/local/bin/haco", "_git-agent")
	return runGitAgentCommand(ctx, cancel, cmd, input, request.PackOutput)
}

func runGitAgentCommand(ctx context.Context, cancel context.CancelFunc, cmd *exec.Cmd, input io.Reader, packOutput io.Writer) (gitadapter.Response, error) {
	cmd.Stdin = input
	diagnostic := gitadapter.NewGitDiagnostic(gitadapter.ProgressWriter(ctx))
	cmd.Stderr = diagnostic
	// Incus forwards interrupt through its exec control channel to the agent.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 5 * time.Second
	output, err := cmd.StdoutPipe()
	if err != nil {
		return gitadapter.Response{}, fmt.Errorf("trusted Git agent unavailable")
	}
	defer func() { _ = output.Close() }()
	if err := cmd.Start(); err != nil {
		return gitadapter.Response{}, fmt.Errorf("trusted Git agent unavailable")
	}
	response, decodeErr := gitadapter.ReadResponse(output, packOutput)
	callerErr := ctx.Err()
	if decodeErr != nil {
		cancel()
	}
	waitErr := cmd.Wait()
	diagnostic.Flush()
	if ctx.Err() != nil && decodeErr == nil {
		return gitadapter.Response{}, ctx.Err()
	}
	if callerErr != nil {
		return gitadapter.Response{}, callerErr
	}
	if decodeErr != nil {
		return gitadapter.Response{}, decodeErr
	}
	if waitErr != nil {
		return gitadapter.Response{}, fmt.Errorf("trusted Git agent unavailable")
	}
	return response, nil
}

func (b *RepositoryBackend) gitConnectionDevices(ctx context.Context, environment core.Environment, workspace gitrepo.Object) (map[string]map[string]string, error) {
	ref := "haco-" + environment.Name
	if err := validateManagedInstanceRef(ref); err != nil {
		return nil, err
	}
	if !environmentapp.MatchesRuntimeRef(environment.RuntimeRef, environmentapp.ProviderIncus, ref) {
		return nil, core.ErrInvalidArgument
	}
	attachments, err := b.WorkspaceAttachments(ctx, workspace)
	if err != nil {
		return nil, err
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/instances/"+ref+"?project="+b.Runtime.project)
	if err != nil {
		return nil, err
	}
	var instance struct {
		Config  map[string]string            `json:"config"`
		Devices map[string]map[string]string `json:"devices"`
	}
	if json.Unmarshal([]byte(result.Stdout), &instance) != nil || instance.Config[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue {
		return nil, core.ErrIncompatibleState
	}
	for _, mount := range attachments {
		disk := instance.Devices[mount.Device]
		if disk["type"] != "disk" || disk["pool"] != mount.Pool || disk["source"] != mount.Volume || disk["path"] != mount.Path {
			return nil, core.ErrIncompatibleState
		}
	}
	return instance.Devices, nil
}

func gitConnectionProxy(socket string) map[string]string {
	return map[string]string{"type": "proxy", "bind": "instance", "listen": "unix:" + gitadapter.GuestSocket, "connect": "unix:" + socket, "mode": "0600", "uid": "0", "gid": "0"}
}

func (b *RepositoryBackend) InspectGitConnection(ctx context.Context, environment core.Environment, workspace gitrepo.Object, socket string) (bool, error) {
	devices, err := b.gitConnectionDevices(ctx, environment, workspace)
	if err != nil {
		return false, err
	}
	existing, ok := devices["git-broker"]
	if !ok {
		return false, nil
	}
	if !reflect.DeepEqual(existing, gitConnectionProxy(socket)) {
		return false, core.ErrIncompatibleState
	}
	return true, nil
}

func (b *RepositoryBackend) ConnectGit(ctx context.Context, environment core.Environment, workspace gitrepo.Object, socket string) error {
	devices, err := b.gitConnectionDevices(ctx, environment, workspace)
	if err != nil {
		return err
	}
	ref := "haco-" + environment.Name
	device := gitConnectionProxy(socket)
	if old, exists := devices["git-broker"]; exists {
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

type WorkspaceAttachment struct{ Device, Pool, Volume, Path, Owner, Repository, Remote, Branch string }

func validWorkspaceAttachment(m WorkspaceAttachment) bool {
	if !safeIncusRef(m.Pool) || !safeIncusRef(m.Volume) {
		return false
	}
	return (m.Device == "workspace" && m.Path == "/workspace") ||
		(strings.HasPrefix(m.Device, "workspace-") && gitadapter.ValidID(strings.TrimPrefix(m.Device, "workspace-")) && m.Path == "/workspace/"+strings.TrimPrefix(m.Device, "workspace-"))
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
		mount := WorkspaceAttachment{Device: "workspace", Pool: pool, Volume: volume, Path: "/workspace", Owner: member.Owner, Repository: member.Repository, Remote: member.Remote, Branch: member.Branch}
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
