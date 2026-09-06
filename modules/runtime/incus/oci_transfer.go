package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/SLktEx/Hacocoon/internal/core"
	environmentapp "github.com/SLktEx/Hacocoon/internal/environment"
	"github.com/SLktEx/Hacocoon/modules/plugin/oci"
)

// OCITransferBackend only moves an image archive. It never mounts sockets,
// credentials, writable stores, or instance root filesystems across boundaries.
type OCITransferBackend struct{ Runtime *Runtime }

func localOCIArgs(driver oci.Driver, operation string, image string) ([]string, error) {
	args := []string{"/usr/bin/env", "-i", "PATH=/usr/local/bin:/usr/bin:/bin", "HOME=/root"}
	switch driver {
	case oci.DriverDocker:
		args = append(args, "docker", "--host", "unix:///run/docker.sock")
	case oci.DriverNerdctl:
		args = append(args, "nerdctl", "--address", "/run/containerd/containerd.sock", "--namespace", "default")
	default:
		return nil, core.ErrInvalidArgument
	}
	if operation != "save" && operation != "load" {
		return nil, core.ErrInvalidArgument
	}
	args = append(args, operation)
	if operation == "save" {
		args = append(args, "--", image)
	}
	return args, nil
}
func (b *OCITransferBackend) SaveImage(ctx context.Context, driver oci.Driver, image string, out io.Writer) error {
	if err := b.Runtime.verifyTrustedHostOwnership(ctx); err != nil {
		return err
	}
	argv, err := localOCIArgs(driver, "save", image)
	if err != nil {
		return err
	}
	return b.run(ctx, trustedHostName, argv, nil, out)
}
func (b *OCITransferBackend) LoadImage(ctx context.Context, env core.Environment, driver oci.Driver, in io.Reader) error {
	ref := "haco-" + env.Name
	if !safeIncusRef(ref) || !environmentapp.MatchesRuntimeRef(env.RuntimeRef, environmentapp.ProviderIncus, ref) {
		return core.ErrInvalidArgument
	}
	result, err := b.Runtime.runner.Run(ctx, "incus", "query", "/1.0/instances/"+ref+"?project="+b.Runtime.project)
	if err != nil || result.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	var observed struct {
		Config map[string]string `json:"expanded_config"`
		Status string            `json:"status"`
	}
	if json.Unmarshal([]byte(result.Stdout), &observed) != nil || observed.Config[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue || observed.Status != "Running" {
		return core.ErrIncompatibleState
	}
	if privileged := observed.Config["security.privileged"]; privileged != "" && privileged != "false" {
		return core.ErrIncompatibleState
	}
	argv, err := localOCIArgs(driver, "load", "")
	if err != nil {
		return err
	}
	return b.run(ctx, ref, argv, in, io.Discard)
}
func (b *OCITransferBackend) run(ctx context.Context, ref string, argv []string, in io.Reader, out io.Writer) error {
	args := append([]string{"exec", ref, "--project", b.Runtime.project, "--disable-stdin=false", "--"}, argv...)
	cmd := exec.CommandContext(ctx, "incus", args...)
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("OCI image transfer process failed: %w", err)
	}
	return nil
}
