package incus

import (
	"context"
	"reflect"
	"regexp"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
)

var hostImageDigest = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var hostContainerID = regexp.MustCompile(`^[a-f0-9]{64}$`)
var hostImageReference = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,999}$`)

// The Host adapter independently restricts commands: this entry must never turn
// image review data into arbitrary Host execution, templates or daemon options.
func validHostImageArguments(args []string) bool {
	if reflect.DeepEqual(args, []string{"image", "ls", "--quiet", "--no-trunc"}) || reflect.DeepEqual(args, []string{"container", "ls", "--all", "--quiet", "--no-trunc"}) {
		return true
	}
	if len(args) == 3 && args[0] == "image" && args[1] == "rm" {
		return hostImageDigest.MatchString(args[2])
	}
	if len(args) == 5 && args[1] == "inspect" && args[2] == "--format" {
		if args[0] == "image" && args[3] == `{{json .Id}} {{json .RepoTags}} {{json .RepoDigests}}` {
			return hostImageDigest.MatchString(args[4])
		}
		if args[0] == "container" && args[3] == `{{json .Image}} {{json .Name}}` {
			return hostContainerID.MatchString(args[4])
		}
	}
	return len(args) == 6 && args[0] == "image" && args[1] == "inspect" && args[2] == "--format" && args[3] == `{{json .RepoDigests}}` && args[4] == "--" && hostImageReference.MatchString(args[5])
}

func (b *PersistentResourceBackend) ExecHostImage(ctx context.Context, source core.PersistentResource, tool string, args []string) (core.ExecutionResult, error) {
	if (tool != "docker" && tool != "nerdctl") || !validHostImageArguments(args) || source.ID != "oci-source:host" || !source.SourceOnly || source.State != "ready" || source.Kind != OCIStoreKind || source.WorkspaceID != "" || !core.ValidPersistentResourceRef(source.Ref()) {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	unlock, err := lockHostOperation(ctx, b.Runtime.project)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	defer unlock()
	if err := b.Runtime.rejectPendingHostCopy(ctx); err != nil {
		return core.ExecutionResult{}, err
	}
	instance, err := b.hostCopyInstance(ctx, source)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if instance.Type != "container" || instance.Profiles == nil || len(instance.Profiles) != 0 || instance.LocalConfig[trustedHostRoleKey] != trustedHostRoleValue || instance.LocalConfig[hostOCIStoreKey] != source.Owner || instance.StatusCode != 103 || instance.Config[hostOCICopyKey] != "" || (instance.Config["security.privileged"] != "" && instance.Config["security.privileged"] != "false") {
		return core.ExecutionResult{}, core.ErrRecoveryRequired
	}
	if err := b.VerifyHostSource(ctx, source); err != nil {
		return core.ExecutionResult{}, err
	}
	argv := []string{"exec", trustedHostName, "--project", b.Runtime.project, "--", "/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/nonexistent"}
	if tool == "docker" {
		argv = append(argv, "docker", "--host", "unix:///run/docker.sock")
	} else {
		argv = append(argv, "nerdctl", "--address", "/run/containerd/containerd.sock", "--namespace", "default", "--snapshotter", "native")
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", append(argv, args...)...)
	if err != nil || result.ExitCode != 0 || result.StdoutTruncated || result.StderrTruncated {
		return core.ExecutionResult{}, core.ErrRuntimeUnavailable
	}
	return core.ExecutionResult{Stdout: result.Stdout, ExitCode: result.ExitCode}, nil
}
