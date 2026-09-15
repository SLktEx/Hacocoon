package incus

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const defaultProject = "hacocoon"

const defaultResourceProject = "default"

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
		return fmt.Errorf("incus environment %s: %w", ref, core.ErrNotFound)
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

func (r *Runtime) ensureProject(ctx context.Context) error {
	if _, err := r.runner.Run(ctx, "incus", "project", "show", r.project); err == nil {
		return nil
	}
	_, err := r.runner.Run(ctx, "incus", "project", "create", r.project, "--config", "features.profiles=false")
	return err
}
