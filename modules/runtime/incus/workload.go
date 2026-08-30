package incus

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const (
	workloadKindKey        = "user.hacocoon.kind"
	workloadKindValue      = "oci-workload"
	workloadEnvironmentKey = "user.hacocoon.environment"
	workloadNameKey        = "user.hacocoon.workload"
	workloadImageKey       = "user.hacocoon.image"
	workloadEphemeralKey   = "user.hacocoon.ephemeral"
)

var (
	workloadTokenPattern  = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?$`)
	workloadEnvKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	workloadSafeArg       = regexp.MustCompile(`^[A-Za-z0-9_./:@%+=,-]+$`)
)

type workloadListEntry struct {
	Name   string            `json:"name"`
	Status string            `json:"status"`
	Config map[string]string `json:"config"`
}

// CreateWorkload launches an OCI application container directly on the one
// Physical-Host Incus daemon. The owning Environment never receives Incus
// authority or a containerd socket.
func (r *Runtime) CreateWorkload(ctx context.Context, spec core.WorkloadSpec) (core.Workload, error) {
	if r == nil || r.runner == nil {
		return core.Workload{}, core.ErrRuntimeUnavailable
	}
	if err := validateWorkloadSpec(spec); err != nil {
		return core.Workload{}, err
	}
	if err := r.verifyManagedEnvironment(ctx, spec.Environment); err != nil {
		return core.Workload{}, err
	}
	if err := r.ensureProject(ctx); err != nil {
		return core.Workload{}, fmt.Errorf("ensure Incus project for OCI workload: %w", err)
	}
	if err := r.ensureSandboxNetwork(ctx); err != nil {
		return core.Workload{}, fmt.Errorf("ensure Incus network for OCI workload: %w", err)
	}
	rootPool, err := r.defaultRootPool(ctx)
	if err != nil {
		return core.Workload{}, fmt.Errorf("resolve OCI workload storage: %w", err)
	}
	ref, err := workloadRef(spec.Environment, spec.Name)
	if err != nil {
		return core.Workload{}, err
	}
	if exists, err := r.environmentExists(ctx, ref); err != nil {
		return core.Workload{}, err
	} else if exists {
		return core.Workload{}, fmt.Errorf("OCI workload %q already exists: %w", spec.Name, core.ErrAlreadyExists)
	}

	args := []string{
		"launch", spec.Image, ref,
		"--project", r.project,
		"--profile", sandboxProfile,
		"--storage", rootPool,
		"--config", workloadKindKey+"="+workloadKindValue,
		"--config", workloadEnvironmentKey+"="+spec.Environment,
		"--config", workloadNameKey+"="+spec.Name,
		"--config", workloadImageKey+"="+spec.Image,
	}
	if spec.Ephemeral {
		args = append(args, "--ephemeral", "--config", workloadEphemeralKey+"=true")
	}
	if len(spec.Command) > 0 {
		entrypoint, err := encodeOCIEntrypoint(spec.Command)
		if err != nil {
			return core.Workload{}, err
		}
		args = append(args, "--config", "oci.entrypoint="+entrypoint)
	}
	keys := make([]string, 0, len(spec.EnvironmentVariables))
	for key := range spec.EnvironmentVariables {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--config", "environment."+key+"="+spec.EnvironmentVariables[key])
	}

	if _, err := r.runner.Run(ctx, "incus", args...); err != nil {
		return core.Workload{}, fmt.Errorf("launch OCI workload %s: %w", ref, err)
	}
	return core.Workload{
		Environment: spec.Environment,
		Name:        spec.Name,
		RuntimeRef:  ref,
		Image:       spec.Image,
		State:       "RUNNING",
		Ephemeral:   spec.Ephemeral,
	}, nil
}

func (r *Runtime) ListWorkloads(ctx context.Context, environment string) ([]core.Workload, error) {
	if err := validateWorkloadToken("environment", environment); err != nil {
		return nil, err
	}
	if err := r.verifyManagedEnvironment(ctx, environment); err != nil {
		return nil, err
	}
	result, err := r.runner.Run(ctx, "incus", "list", "--project", r.project, "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("list Incus OCI workloads: %w", err)
	}
	var entries []workloadListEntry
	if err := json.Unmarshal([]byte(result.Stdout), &entries); err != nil {
		return nil, fmt.Errorf("decode Incus OCI workload inventory: %w", core.ErrIncompatibleState)
	}
	workloads := make([]core.Workload, 0)
	for _, entry := range entries {
		if entry.Config[workloadKindKey] != workloadKindValue || entry.Config[workloadEnvironmentKey] != environment {
			continue
		}
		name := entry.Config[workloadNameKey]
		ref, refErr := workloadRef(environment, name)
		if refErr != nil || ref != entry.Name {
			return nil, fmt.Errorf("OCI workload ownership metadata drifted for %q: %w", entry.Name, core.ErrIncompatibleState)
		}
		workloads = append(workloads, core.Workload{
			Environment: environment,
			Name:        name,
			RuntimeRef:  entry.Name,
			Image:       entry.Config[workloadImageKey],
			State:       strings.ToUpper(strings.TrimSpace(entry.Status)),
			Ephemeral:   entry.Config[workloadEphemeralKey] == "true",
		})
	}
	sort.Slice(workloads, func(i, j int) bool { return workloads[i].Name < workloads[j].Name })
	return workloads, nil
}

func (r *Runtime) ExecWorkload(ctx context.Context, environment, name string, argv []string) (core.ExecutionResult, error) {
	ref, err := r.verifyWorkload(ctx, environment, name)
	if err != nil {
		return core.ExecutionResult{}, err
	}
	if len(argv) == 0 {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	args := append([]string{"exec", ref, "--project", r.project, "--"}, argv...)
	result, runErr := r.runner.Run(ctx, "incus", args...)
	return core.ExecutionResult{ExitCode: result.ExitCode, Stdout: result.Stdout, Stderr: result.Stderr}, runErr
}

func (r *Runtime) StopWorkload(ctx context.Context, environment, name string) error {
	ref, err := r.verifyWorkload(ctx, environment, name)
	if err != nil {
		return err
	}
	if _, err := r.runner.Run(ctx, "incus", "stop", ref, "--project", r.project); err != nil {
		return fmt.Errorf("stop OCI workload %s: %w", ref, err)
	}
	return nil
}

func (r *Runtime) DeleteWorkload(ctx context.Context, environment, name string) error {
	ref, err := r.verifyWorkload(ctx, environment, name)
	if err != nil {
		return err
	}
	if _, err := r.runner.Run(ctx, "incus", "delete", ref, "--project", r.project, "--force"); err != nil {
		return fmt.Errorf("delete OCI workload %s: %w", ref, err)
	}
	return nil
}

// PullWorkloadImage explicitly warms the local Incus image store. The source
// must be an Incus image reference (for example oci-docker:library/postgres:18
// or an operator-configured private OCI remote). Credential transport is kept
// out of this method so reusable registry credentials never enter Incus state.
func (r *Runtime) PullWorkloadImage(ctx context.Context, image string) error {
	if err := validateWorkloadImage(image); err != nil {
		return err
	}
	if err := r.ensureProject(ctx); err != nil {
		return err
	}
	if _, err := r.runner.Run(ctx, "incus", "image", "copy", image, "local:",
		"--project", r.project,
		"--target-project", r.project,
	); err != nil {
		return fmt.Errorf("pull OCI image %q into Incus cache: %w", image, err)
	}
	return nil
}

func (r *Runtime) verifyManagedEnvironment(ctx context.Context, environment string) error {
	if err := validateWorkloadToken("environment", environment); err != nil {
		return err
	}
	ref := "haco-" + environment
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	exists, err := r.environmentExists(ctx, ref)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("Environment %q does not exist: %w", environment, core.ErrNotFound)
	}
	result, err := r.runner.Run(ctx, "incus", "config", "get", ref, managedEnvironmentMarkerKey, "--project", r.project)
	if err != nil {
		return fmt.Errorf("verify Environment ownership for OCI workload: %w", err)
	}
	if strings.TrimSpace(result.Stdout) != managedEnvironmentMarkerValue {
		return fmt.Errorf("Incus instance %q is not a managed Hacocoon Environment: %w", ref, core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) verifyWorkload(ctx context.Context, environment, name string) (string, error) {
	ref, err := workloadRef(environment, name)
	if err != nil {
		return "", err
	}
	exists, err := r.environmentExists(ctx, ref)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("OCI workload %q does not exist: %w", name, core.ErrNotFound)
	}
	checks := map[string]string{
		workloadKindKey:        workloadKindValue,
		workloadEnvironmentKey: environment,
		workloadNameKey:        name,
	}
	for key, expected := range checks {
		result, err := r.runner.Run(ctx, "incus", "config", "get", ref, key, "--project", r.project)
		if err != nil {
			return "", fmt.Errorf("verify OCI workload ownership %s: %w", key, err)
		}
		if strings.TrimSpace(result.Stdout) != expected {
			return "", fmt.Errorf("OCI workload %q ownership metadata drifted: %w", ref, core.ErrIncompatibleState)
		}
	}
	return ref, nil
}

func workloadRef(environment, name string) (string, error) {
	if err := validateWorkloadToken("environment", environment); err != nil {
		return "", err
	}
	if err := validateWorkloadToken("workload", name); err != nil {
		return "", err
	}
	ref := "haco-w-" + environment + "-" + name
	if err := validateManagedInstanceRef(ref); err != nil {
		return "", fmt.Errorf("OCI workload identity is too long or invalid: %w", err)
	}
	return ref, nil
}

func validateWorkloadSpec(spec core.WorkloadSpec) error {
	if err := validateWorkloadToken("environment", spec.Environment); err != nil {
		return err
	}
	if err := validateWorkloadToken("workload", spec.Name); err != nil {
		return err
	}
	if err := validateWorkloadImage(spec.Image); err != nil {
		return err
	}
	for key, value := range spec.EnvironmentVariables {
		if !workloadEnvKeyPattern.MatchString(key) || hasWorkloadControl(value) {
			return fmt.Errorf("invalid OCI workload environment variable %q: %w", key, core.ErrInvalidArgument)
		}
	}
	for _, arg := range spec.Command {
		if hasWorkloadControl(arg) {
			return fmt.Errorf("OCI workload command contains control characters: %w", core.ErrInvalidArgument)
		}
	}
	_, err := workloadRef(spec.Environment, spec.Name)
	return err
}

func validateWorkloadToken(kind, value string) error {
	if strings.TrimSpace(value) != value || !workloadTokenPattern.MatchString(value) {
		return fmt.Errorf("invalid OCI %s %q: %w", kind, value, core.ErrInvalidArgument)
	}
	return nil
}

func validateWorkloadImage(image string) error {
	if image == "" || strings.TrimSpace(image) != image || strings.HasPrefix(image, "-") || hasWorkloadControl(image) {
		return fmt.Errorf("invalid OCI image reference %q: %w", image, core.ErrInvalidArgument)
	}
	return nil
}

func hasWorkloadControl(value string) bool {
	return strings.ContainsAny(value, "\x00\r\n")
}

func encodeOCIEntrypoint(argv []string) (string, error) {
	if len(argv) == 0 {
		return "", core.ErrInvalidArgument
	}
	encoded := make([]string, 0, len(argv))
	for _, arg := range argv {
		if hasWorkloadControl(arg) {
			return "", core.ErrInvalidArgument
		}
		if workloadSafeArg.MatchString(arg) {
			encoded = append(encoded, arg)
			continue
		}
		encoded = append(encoded, "'"+strings.ReplaceAll(arg, "'", "'\"'\"'")+"'")
	}
	return strings.Join(encoded, " "), nil
}
