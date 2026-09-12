package incus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const defaultProject = "hacocoon"

const defaultImage = "images:ubuntu/26.04"

const defaultCleanupTimeout = 30 * time.Second

type runtimeStorageState struct {
	mu       sync.Mutex
	rootPool string
	provider func(context.Context) (map[string]string, error)
}

type Runtime struct {
	maintenanceTooling       func(context.Context) (string, func() error, error)
	environmentDNS           string
	trustedHostInterop       func(context.Context) error
	trustedHostNotifications func(context.Context) error
	trustedHostStorage       func(context.Context) error
	trustedHostCopyRecovery  func(context.Context) error
	runner                   host.Runner
	project                  string
	image                    string
	storage                  *runtimeStorageState
	stdin                    io.Reader
	stdout                   io.Writer
	stderr                   io.Writer
	cleanupTimeout           time.Duration
	managedWorkspace         func(context.Context, string) ([]WorkspaceAttachment, error)
}

func New(runner host.Runner) *Runtime {
	return &Runtime{
		runner:         runner,
		project:        defaultProject,
		image:          defaultImage,
		storage:        &runtimeStorageState{},
		stdin:          os.Stdin,
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		cleanupTimeout: defaultCleanupTimeout,
	}
}

func (*Runtime) ID() string { return "runtime.incus" }

func (r *Runtime) Prepare(ctx context.Context, spec core.RuntimePrepareSpec) error {
	if err := r.ensureProject(ctx); err != nil {
		return err
	}
	if err := r.ensureSandboxNetwork(ctx); err != nil {
		return fmt.Errorf("ensure Hacocoon sandbox network: %w", err)
	}
	pool, err := r.ensureStoragePool(ctx, spec.StorageAttachment)
	if err != nil {
		return err
	}
	if pool != "" {
		r.setRootPool(pool)
	}
	return nil
}

func (r *Runtime) Create(ctx context.Context, spec core.RuntimeSessionSpec) (core.RuntimeSession, error) {
	if err := r.ensureProject(ctx); err != nil {
		return core.RuntimeSession{}, err
	}
	if err := r.ensureSandboxNetwork(ctx); err != nil {
		return core.RuntimeSession{}, fmt.Errorf("ensure Hacocoon sandbox network: %w", err)
	}
	pool, err := r.ensureStoragePool(ctx, spec.StorageAttachment)
	if err != nil {
		return core.RuntimeSession{}, err
	}
	name := "haco-" + string(spec.ID)
	if err := validateManagedInstanceRef(name); err != nil {
		return core.RuntimeSession{}, err
	}
	args := []string{"launch", r.image, name, "--project", r.project, "--profile", sandboxProfile, "--config", "boot.autostart=false"}
	if pool != "" {
		args = append(args, "--storage", pool)
	}
	if _, err := r.runner.Run(ctx, "incus", args...); err != nil {
		return core.RuntimeSession{}, err
	}
	return core.RuntimeSession{Ref: name}, nil
}

func (r *Runtime) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	// Retained Store startup is not wired yet. Never fall through to ordinary
	// creation, which may start daemons before maintenance preparation.
	if spec.ResourceMaintenance {
		return core.EnvironmentRuntime{}, core.ErrUnsupported
	}
	if spec.Name == "" || spec.WorkspacePath == "" {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	identityArgs, err := environmentIdentityArgs(spec.InstanceID)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	ref := "haco-" + spec.Name
	if ref == trustedHostName {
		return core.EnvironmentRuntime{}, fmt.Errorf("environment name %q is reserved for trusted Hacocoon infrastructure: %w", spec.Name, core.ErrInvalidArgument)
	}
	if err := validateManagedInstanceRef(ref); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if err := r.ensureProject(ctx); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("ensure Incus project: %w", err)
	}
	rootPool, err := r.defaultRootPool(ctx)
	if err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("resolve isolated root storage: %w", err)
	}

	initArgs := append([]string{"init", r.image, ref, "--project", r.project, "--profile", sandboxProfile, "--storage", rootPool, "--config", "boot.autostart=false"}, identityArgs...)
	if _, err := r.runner.Run(ctx, "incus", initArgs...); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("init isolated Incus environment %s: %w", ref, err)
	}
	cleanup := func(cause error) (core.EnvironmentRuntime, error) {
		return r.cleanupFailedEnvironment(ctx, ref, cause, r.DeleteEnvironment)
	}

	// Profiles are intentionally shared from the default project, but the
	// network device is an authority boundary and must not rely solely on
	// cross-project profile expansion. Copy the inherited NIC into the instance
	// and pin every security-sensitive property before the environment starts.
	if err := r.materializeSandboxNIC(ctx, ref); err != nil {
		return cleanup(fmt.Errorf("materialize sandbox NIC in %s: %w", ref, err))
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

func (r *Runtime) DeleteEnvironment(ctx context.Context, ref string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	result, err := r.runner.Run(ctx, "incus", "delete", ref, "--project", r.project, "--force")
	if ctx.Err() != nil {
		return errors.Join(err, ctx.Err())
	}
	if err == nil && result.ExitCode == 0 {
		return nil
	}
	if err == nil {
		err = core.ErrRuntimeUnavailable
	}
	if ctx.Err() != nil {
		return err
	}
	exists, inspectErr := r.environmentExists(ctx, ref)
	if inspectErr != nil {
		return errors.Join(err, fmt.Errorf("confirm Incus delete state for %s: %w", ref, inspectErr))
	}
	if !exists {
		return fmt.Errorf("Incus environment %s: %w", ref, core.ErrNotFound)
	}
	return err
}

func (r *Runtime) Start(ctx context.Context, ref string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	_, err := r.runner.Run(ctx, "incus", "start", ref, "--project", r.project)
	return err
}

func (r *Runtime) Stop(ctx context.Context, ref string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	_, err := r.runner.Run(ctx, "incus", "stop", ref, "--project", r.project)
	return err
}

func (r *Runtime) Delete(ctx context.Context, ref string) error {
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	_, err := r.runner.Run(ctx, "incus", "delete", ref, "--project", r.project, "--force")
	return err
}

func (r *Runtime) materializeSandboxNIC(ctx context.Context, ref string) error {
	args := []string{"config", "device", "override", ref, "eth0"}
	for _, key := range []string{
		"name",
		"network",
		"security.ipv4_filtering",
		"security.ipv6_filtering",
		"security.mac_filtering",
		"security.port_isolation",
	} {
		args = append(args, key+"="+sandboxNIC[key])
	}
	args = append(args, "--project", r.project)
	result, err := r.runner.Run(ctx, "incus", args...)
	if err != nil {
		reason := strings.TrimSpace(result.Stderr)
		if reason != "" {
			return fmt.Errorf("pin inherited sandbox NIC: %s: %w", reason, err)
		}
		return fmt.Errorf("pin inherited sandbox NIC: %w", err)
	}
	return nil
}

func (r *Runtime) ensureProject(ctx context.Context) error {
	if _, err := r.runner.Run(ctx, "incus", "project", "show", r.project); err == nil {
		return nil
	}
	_, err := r.runner.Run(ctx, "incus", "project", "create", r.project, "--config", "features.profiles=false")
	return err
}
