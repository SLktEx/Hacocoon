package incus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const defaultProject = "hacocoon"

const defaultImage = "images:ubuntu/26.04"

const defaultCleanupTimeout = 30 * time.Second

type Runtime struct {
	runner         host.Runner
	project        string
	image          string
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
	cleanupTimeout time.Duration
}

func New(runner host.Runner) *Runtime {
	return &Runtime{
		runner:         runner,
		project:        defaultProject,
		image:          defaultImage,
		stdin:          os.Stdin,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		cleanupTimeout: defaultCleanupTimeout,
	}
}

func (*Runtime) ID() string { return "runtime.incus" }

func (r *Runtime) Probe(ctx context.Context) (core.RuntimeCapabilities, error) {
	result, err := r.runner.Run(ctx, "incus", "version")
	if err != nil {
		return core.RuntimeCapabilities{Available: false, Details: []string{"incus unavailable"}}, nil
	}
	return core.RuntimeCapabilities{Available: true, Details: []string{strings.TrimSpace(result.Stdout)}}, nil
}

func (r *Runtime) Prepare(ctx context.Context, spec core.RuntimePrepareSpec) error {
	if err := r.ensureProject(ctx); err != nil {
		return err
	}
	_, err := r.ensureStoragePool(ctx, spec.StorageAttachment)
	return err
}

func (r *Runtime) Create(ctx context.Context, spec core.RuntimeSessionSpec) (core.RuntimeSession, error) {
	if err := r.ensureProject(ctx); err != nil {
		return core.RuntimeSession{}, err
	}
	pool, err := r.ensureStoragePool(ctx, spec.StorageAttachment)
	if err != nil {
		return core.RuntimeSession{}, err
	}
	name := "haco-" + string(spec.ID)
	args := []string{"launch", r.image, name, "--project", r.project}
	if pool != "" {
		args = append(args, "--storage", pool)
	}
	if _, err := r.runner.Run(ctx, "incus", args...); err != nil {
		return core.RuntimeSession{}, err
	}
	return core.RuntimeSession{Ref: name}, nil
}

func (r *Runtime) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if spec.Name == "" || spec.WorkspacePath == "" {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	if err := r.ensureProject(ctx); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("ensure Incus project: %w", err)
	}
	rootPool, err := r.defaultRootPool(ctx)
	if err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("resolve isolated root storage: %w", err)
	}

	ref := "haco-" + spec.Name
	if _, err := r.runner.Run(ctx, "incus", "init", r.image, ref, "--project", r.project, "--no-profiles", "--storage", rootPool); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("init isolated Incus environment %s: %w", ref, err)
	}
	cleanup := func(cause error) (core.EnvironmentRuntime, error) {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.cleanupTimeout)
		defer cancel()
		result, cleanupErr := r.runner.Run(cleanupCtx, "incus", "delete", ref, "--project", r.project, "--force")
		if cleanupErr != nil && !isIncusNotFound(result) {
			return core.EnvironmentRuntime{}, errors.Join(
				cause,
				fmt.Errorf("cleanup Incus environment %s: %w", ref, cleanupErr),
				core.ErrRecoveryRequired,
			)
		}
		return core.EnvironmentRuntime{}, cause
	}

	deviceArgs := []string{
		"config", "device", "add", ref, "workspace", "disk",
		"source=" + spec.WorkspacePath,
		"path=/workspace",
	}
	if spec.ReadOnly {
		deviceArgs = append(deviceArgs, "readonly=true")
	} else {
		deviceArgs = append(deviceArgs, "shift=true")
	}
	deviceArgs = append(deviceArgs, "--project", r.project)
	if _, err := r.runner.Run(ctx, "incus", deviceArgs...); err != nil {
		return cleanup(fmt.Errorf("mount workspace in %s: %w", ref, err))
	}
	if _, err := r.runner.Run(ctx, "incus", "start", ref, "--project", r.project); err != nil {
		return cleanup(fmt.Errorf("start Incus environment %s: %w", ref, err))
	}
	if !spec.ReadOnly {
		result, err := r.runner.Run(ctx, "incus", "exec", ref, "--project", r.project, "--", "test", "-w", "/workspace")
		if err != nil {
			reason := strings.TrimSpace(result.Stderr)
			if reason == "" {
				reason = err.Error()
			}
			return cleanup(errors.Join(
				fmt.Errorf("workspace %q is not writable from unprivileged environment %s: %s", spec.WorkspacePath, ref, reason),
				core.ErrUnsupported,
			))
		}
	}
	return core.EnvironmentRuntime{Ref: ref}, nil
}

func (r *Runtime) ExecEnvironment(ctx context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if len(req.Argv) == 0 {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	args := append([]string{"exec", ref, "--project", r.project, "--"}, req.Argv...)
	result, err := r.runner.Run(ctx, "incus", args...)
	return core.ExecutionResult{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, err
}

func (r *Runtime) ShellEnvironment(ctx context.Context, ref string) error {
	_, err := r.execInteractive(ctx, ref, []string{"/bin/bash"})
	return err
}

func (r *Runtime) DeleteEnvironment(ctx context.Context, ref string) error {
	result, err := r.runner.Run(ctx, "incus", "delete", ref, "--project", r.project, "--force")
	if err != nil && isIncusNotFound(result) {
		return fmt.Errorf("Incus environment %s: %w", ref, core.ErrNotFound)
	}
	return err
}

func (r *Runtime) InspectEnvironment(ctx context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	result, err := r.runner.Run(ctx, "incus", "list", ref, "--project", r.project, "--format", "csv", "-c", "s")
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	states := map[string]core.EnvironmentState{
		"RUNNING": core.EnvironmentRunning,
		"STOPPED": core.EnvironmentStopped,
	}
	state, ok := states[strings.ToUpper(strings.TrimSpace(result.Stdout))]
	if !ok {
		state = core.EnvironmentUnknown
	}
	return core.EnvironmentRuntimeStatus{State: state}, nil
}

func (r *Runtime) ForwardLocalPort(ctx context.Context, ref string, req core.LocalPortRequest) (core.ClientConnection, error) {
	id := fmt.Sprintf("tcp-%d-%d", req.HostPort, req.TargetPort)
	if err := r.addLoopbackProxy(ctx, ref, id, req.HostPort, req.TargetPort); err != nil {
		return core.ClientConnection{}, err
	}
	return core.ClientConnection{ID: id, Kind: "tcp", Host: "127.0.0.1", Port: req.HostPort, TargetPort: req.TargetPort}, nil
}

func (r *Runtime) RemoveClientConnection(ctx context.Context, ref, connectionID string) error {
	_, err := r.runner.Run(ctx, "incus", "config", "device", "remove", ref, "haco-"+connectionID, "--project", r.project)
	return err
}

func (r *Runtime) PrepareSSH(ctx context.Context, ref string, req core.SSHAccessRequest) (core.ClientConnection, error) {
	return r.PrepareSSHAccess(ctx, ref, req)
}

func (r *Runtime) addLoopbackProxy(ctx context.Context, ref, id string, hostPort, targetPort int) error {
	_, err := r.runner.Run(ctx, "incus", "config", "device", "add", ref, "haco-"+id, "proxy",
		fmt.Sprintf("listen=tcp:127.0.0.1:%d", hostPort),
		fmt.Sprintf("connect=tcp:127.0.0.1:%d", targetPort),
		"--project", r.project)
	if err != nil {
		return fmt.Errorf("add local proxy %s: %w", id, err)
	}
	return nil
}

func (r *Runtime) Start(ctx context.Context, ref string) error {
	_, err := r.runner.Run(ctx, "incus", "start", ref, "--project", r.project)
	return err
}

func (r *Runtime) Stop(ctx context.Context, ref string) error {
	_, err := r.runner.Run(ctx, "incus", "stop", ref, "--project", r.project)
	return err
}

func (r *Runtime) Delete(ctx context.Context, ref string) error {
	_, err := r.runner.Run(ctx, "incus", "delete", ref, "--project", r.project, "--force")
	return err
}

func (r *Runtime) Exec(ctx context.Context, ref string, req core.ExecRequest) (core.ExecResult, error) {
	if len(req.Argv) == 0 {
		return core.ExecResult{}, core.ErrInvalidArgument
	}
	if req.Interactive {
		return r.execInteractive(ctx, ref, req.Argv)
	}
	args := append([]string{"exec", ref, "--project", r.project, "--"}, req.Argv...)
	result, err := r.runner.Run(ctx, "incus", args...)
	return core.ExecResult{ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr}, err
}

func (r *Runtime) Inspect(ctx context.Context, ref string) (core.RuntimeState, error) {
	result, err := r.runner.Run(ctx, "incus", "list", ref, "--project", r.project, "--format", "csv", "-c", "s")
	if err != nil {
		return core.RuntimeState{}, err
	}
	states := map[string]core.ObservedState{
		"RUNNING": core.ObservedRunning,
		"STOPPED": core.ObservedStopped,
	}
	state, ok := states[strings.ToUpper(strings.TrimSpace(result.Stdout))]
	if !ok {
		state = core.ObservedUnknown
	}
	return core.RuntimeState{Observed: state}, nil
}

func (r *Runtime) ensureProject(ctx context.Context) error {
	if _, err := r.runner.Run(ctx, "incus", "project", "show", r.project); err == nil {
		return nil
	}
	_, err := r.runner.Run(ctx, "incus", "project", "create", r.project, "--config", "features.profiles=false")
	return err
}

func (r *Runtime) defaultRootPool(ctx context.Context) (string, error) {
	result, err := r.runner.Run(ctx, "incus", "profile", "show", "default", "--project", "default", "--format", "json")
	if err != nil {
		return "", err
	}
	var profile struct {
		Devices map[string]map[string]string `json:"devices"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &profile); err != nil {
		return "", fmt.Errorf("decode default profile: %w", err)
	}
	for _, device := range profile.Devices {
		if device["type"] == "disk" && device["path"] == "/" && device["pool"] != "" {
			return device["pool"], nil
		}
	}
	return "", fmt.Errorf("default profile has no root disk pool: %w", core.ErrUnsupported)
}

func (r *Runtime) ensureStoragePool(ctx context.Context, attachment map[string]string) (string, error) {
	pool := attachment["incus_pool"]
	if pool == "" {
		return "", nil
	}
	if _, err := r.runner.Run(ctx, "incus", "storage", "show", pool, "--project", r.project); err == nil {
		return pool, nil
	}
	driver := attachment["driver"]
	source := attachment["source"]
	if driver == "" || source == "" {
		return "", fmt.Errorf("storage attachment missing driver/source")
	}
	_, err := r.runner.Run(ctx, "incus", "storage", "create", pool, driver, "source="+source, "--project", r.project)
	return pool, err
}

func (r *Runtime) execInteractive(ctx context.Context, ref string, argv []string) (core.ExecResult, error) {
	args := append([]string{"exec", ref, "--project", r.project, "--"}, argv...)
	cmd := exec.CommandContext(ctx, "incus", args...)
	cmd.Stdin = r.stdin
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	err := cmd.Run()
	if err == nil {
		return core.ExecResult{ExitCode: 0}, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return core.ExecResult{ExitCode: exit.ExitCode()}, err
	}
	return core.ExecResult{ExitCode: -1}, err
}

func isIncusNotFound(result host.Result) bool {
	message := strings.ToLower(result.Stderr + "\n" + result.Stdout)
	return strings.Contains(message, "not found") || strings.Contains(message, "does not exist") || strings.Contains(message, "doesn't exist")
}
