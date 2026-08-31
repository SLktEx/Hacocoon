package incus

import (
	"context"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

// ScopedWorkloads is the Host-owned authority handed to one Environment
// workload broker. Both Environment and Incus Project are captured when the
// broker is created; neither value is accepted from the guest as authority.
//
// The current runtime keeps managed Environments in one configured Incus
// project. Keeping the Project in this scope explicitly makes that invariant
// fail closed and allows a future project-per-Environment layout without
// widening the guest protocol.
type ScopedWorkloads struct {
	runtime     *Runtime
	environment string
	project     string
}

// BindEnvironmentWorkloads creates the authority scope for one guest before
// the Environment itself is launched. This is intentionally side-effect free:
// Environment creation needs the Host broker socket to exist before Incus can
// attach the guest-side proxy device.
func (r *Runtime) BindEnvironmentWorkloads(environment string) (*ScopedWorkloads, error) {
	if r == nil || r.runner == nil {
		return nil, core.ErrRuntimeUnavailable
	}
	if err := validateWorkloadToken("environment", environment); err != nil {
		return nil, err
	}
	project := strings.TrimSpace(r.project)
	if project == "" || project != r.project || strings.ContainsAny(project, "\x00\r\n") {
		return nil, fmt.Errorf("invalid Incus project for Environment workload scope: %w", core.ErrIncompatibleState)
	}
	return &ScopedWorkloads{runtime: r, environment: environment, project: project}, nil
}

// Project returns the Host-derived Incus project for diagnostics and tests.
// It is never serialized into the guest workload API.
func (s *ScopedWorkloads) Project() string {
	if s == nil {
		return ""
	}
	return s.project
}

func (s *ScopedWorkloads) authorize(ctx context.Context, environment string) error {
	if s == nil || s.runtime == nil {
		return core.ErrRuntimeUnavailable
	}
	if environment != s.environment {
		return fmt.Errorf("guest workload scope cannot select Environment %q from %q: %w", environment, s.environment, core.ErrPolicyDenied)
	}
	// The Runtime project is Host configuration, not guest input. If it ever
	// changes underneath an existing listener, refuse the request rather than
	// silently launching into the new project.
	if strings.TrimSpace(s.runtime.project) != s.project {
		return fmt.Errorf("guest workload Incus project changed from %q to %q: %w", s.project, s.runtime.project, core.ErrPolicyDenied)
	}
	return s.runtime.verifyManagedEnvironmentInProject(ctx, s.environment, s.project)
}

func (s *ScopedWorkloads) CreateWorkload(ctx context.Context, spec core.WorkloadSpec) (core.Workload, error) {
	if spec.Environment != "" && spec.Environment != s.environment {
		return core.Workload{}, fmt.Errorf("guest workload request selected Environment %q outside bound scope %q: %w", spec.Environment, s.environment, core.ErrPolicyDenied)
	}
	if err := s.authorize(ctx, s.environment); err != nil {
		return core.Workload{}, err
	}
	spec.Environment = s.environment
	return s.runtime.CreateWorkload(ctx, spec)
}

func (s *ScopedWorkloads) ListWorkloads(ctx context.Context, environment string) ([]core.Workload, error) {
	if err := s.authorize(ctx, environment); err != nil {
		return nil, err
	}
	return s.runtime.ListWorkloads(ctx, s.environment)
}

func (s *ScopedWorkloads) ExecWorkload(ctx context.Context, environment, name string, argv []string) (core.ExecutionResult, error) {
	if err := s.authorize(ctx, environment); err != nil {
		return core.ExecutionResult{}, err
	}
	return s.runtime.ExecWorkload(ctx, s.environment, name, argv)
}

func (s *ScopedWorkloads) StopWorkload(ctx context.Context, environment, name string) error {
	if err := s.authorize(ctx, environment); err != nil {
		return err
	}
	return s.runtime.StopWorkload(ctx, s.environment, name)
}

func (s *ScopedWorkloads) DeleteWorkload(ctx context.Context, environment, name string) error {
	if err := s.authorize(ctx, environment); err != nil {
		return err
	}
	return s.runtime.DeleteWorkload(ctx, s.environment, name)
}

func (s *ScopedWorkloads) PullWorkloadImage(ctx context.Context, image string) error {
	if err := s.authorize(ctx, s.environment); err != nil {
		return err
	}
	return s.runtime.PullWorkloadImage(ctx, image)
}

func (r *Runtime) verifyManagedEnvironmentInProject(ctx context.Context, environment, project string) error {
	if r == nil || r.runner == nil {
		return core.ErrRuntimeUnavailable
	}
	if err := validateWorkloadToken("environment", environment); err != nil {
		return err
	}
	if strings.TrimSpace(project) == "" || strings.TrimSpace(project) != project || strings.ContainsAny(project, "\x00\r\n") {
		return core.ErrInvalidArgument
	}
	ref := "haco-" + environment
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	result, err := r.runner.Run(ctx, "incus", "list", ref, "--project", project, "--format", "csv", "-c", "n")
	if err != nil {
		return fmt.Errorf("inspect Environment %q in Incus project %q: %w", environment, project, err)
	}
	found := false
	for _, line := range strings.Split(result.Stdout, "\n") {
		if strings.TrimSpace(line) == ref {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("Environment %q is not present in bound Incus project %q: %w", environment, project, core.ErrPolicyDenied)
	}
	marker, err := r.runner.Run(ctx, "incus", "config", "get", ref, managedEnvironmentMarkerKey, "--project", project)
	if err != nil {
		return fmt.Errorf("verify Environment ownership in Incus project %q: %w", project, err)
	}
	if strings.TrimSpace(marker.Stdout) != managedEnvironmentMarkerValue {
		return fmt.Errorf("Incus instance %q in project %q is not a managed Hacocoon Environment: %w", ref, project, core.ErrPolicyDenied)
	}
	return nil
}
