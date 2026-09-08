package incus

import (
	"context"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

// LocalOCIImages reads only the named local engine in the owned logical Host.
// Fixed endpoints and a clean process environment exclude ambient remote Docker
// contexts, credential helpers and user containerd namespace/config overrides.
func (r *Runtime) LocalOCIImages(ctx context.Context, driver string) (host.Result, error) {
	var command []string
	switch driver {
	case "docker":
		command = []string{"docker", "--host=unix:///var/run/docker.sock"}
	case "nerdctl":
		command = []string{"nerdctl", "--address=/run/containerd/containerd.sock", "--namespace=default"}
	default:
		return host.Result{}, core.ErrInvalidArgument
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := r.verifyTrustedHostOwnership(ctx); err != nil {
		return host.Result{}, err
	}
	args := []string{"exec", trustedHostName, "--project", r.project, "--cwd", "/", "--", "/usr/bin/env", "-i", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/", "DOCKER_CONFIG=/proc/self", "NERDCTL_TOML=/dev/null"}
	args = append(args, command...)
	args = append(args, "images", "--no-trunc", "--digests", "--format", "{{.Repository}}\t{{.Tag}}\t{{.ID}}\t{{.Digest}}")
	return r.runner.Run(ctx, "incus", args...)
}
