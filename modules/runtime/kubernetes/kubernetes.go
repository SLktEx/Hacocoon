package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	ProviderID            = "runtime.kubernetes"
	defaultKubectl        = "kubectl"
	defaultRuntimeClass   = "sysbox-runc"
	defaultStartupTimeout = 2 * time.Minute
	podName               = "environment"
	managedByLabel        = "app.kubernetes.io/managed-by"
	managedByValue        = "hacocoon"
	roleLabel             = "hacocoon.dev/role"
	roleEnvironment       = "environment"
	environmentLabel      = "hacocoon.dev/environment"
)

var environmentNameRE = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$`)

type Config struct {
	Image          string
	RuntimeClass   string
	Kubectl        string
	StartupTimeout time.Duration
}

type Provider struct {
	runner         host.Runner
	image          string
	runtimeClass   string
	kubectl        string
	startupTimeout time.Duration
	interactive    func(context.Context, string, string) error
}

func New(runner host.Runner, config Config) (*Provider, error) {
	if runner == nil {
		return nil, core.ErrInvalidArgument
	}
	image := strings.TrimSpace(config.Image)
	if image == "" || strings.ContainsAny(image, "\r\n\x00") {
		return nil, fmt.Errorf("Kubernetes Environment image is required: %w", core.ErrInvalidArgument)
	}
	runtimeClass := strings.TrimSpace(config.RuntimeClass)
	if runtimeClass == "" {
		runtimeClass = defaultRuntimeClass
	}
	if !validDNSSubdomain(runtimeClass) {
		return nil, fmt.Errorf("invalid Kubernetes RuntimeClass %q: %w", runtimeClass, core.ErrInvalidArgument)
	}
	kubectl := strings.TrimSpace(config.Kubectl)
	if kubectl == "" {
		kubectl = defaultKubectl
	}
	if filepath.Base(kubectl) != kubectl || strings.ContainsAny(kubectl, "\r\n\x00") {
		return nil, fmt.Errorf("invalid kubectl executable %q: %w", kubectl, core.ErrInvalidArgument)
	}
	timeout := config.StartupTimeout
	if timeout <= 0 {
		timeout = defaultStartupTimeout
	}
	p := &Provider{
		runner:         runner,
		image:          image,
		runtimeClass:   runtimeClass,
		kubectl:        kubectl,
		startupTimeout: timeout,
	}
	p.interactive = p.execInteractive
	return p, nil
}

func (*Provider) ID() string { return ProviderID }

func (p *Provider) Probe(ctx context.Context) (core.RuntimeCapabilities, error) {
	if p == nil || p.runner == nil {
		return core.RuntimeCapabilities{Available: false, Details: []string{"kubernetes provider is not configured"}}, nil
	}
	if _, err := p.runner.Run(ctx, p.kubectl, "version", "--client", "--output=json"); err != nil {
		return core.RuntimeCapabilities{Available: false, Details: []string{"kubectl unavailable"}}, nil
	}
	if _, err := p.runner.Run(ctx, p.kubectl, "get", "runtimeclass", p.runtimeClass, "-o", "name"); err != nil {
		return core.RuntimeCapabilities{Available: false, Details: []string{"Sysbox RuntimeClass unavailable"}}, nil
	}
	return core.RuntimeCapabilities{Available: true, Details: []string{"runtimeClass=" + p.runtimeClass}}, nil
}

func (p *Provider) CreateEnvironment(ctx context.Context, spec core.EnvironmentRuntimeSpec) (core.EnvironmentRuntime, error) {
	if p == nil || p.runner == nil {
		return core.EnvironmentRuntime{}, core.ErrRuntimeUnavailable
	}
	if err := validateEnvironmentName(spec.Name); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if spec.Base != "" {
		return core.EnvironmentRuntime{}, fmt.Errorf("Kubernetes Base selection is not implemented yet: %w", core.ErrUnsupported)
	}
	workspace, err := validateWorkspacePath(spec.WorkspacePath)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	resources, err := core.ResolveResourceBudget(spec.Resources)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if resources.PIDs.Mode == core.ResourceLimitFinite {
		return core.EnvironmentRuntime{}, fmt.Errorf("Kubernetes provider cannot enforce a per-Environment PID limit with the portable Pod API: %w", core.ErrUnsupported)
	}

	ref := namespaceFor(spec.Name)
	existing, err := p.namespace(ctx, ref)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if existing != nil {
		if err := validateOwnedNamespace(existing, ref, spec.Name); err != nil {
			return core.EnvironmentRuntime{}, err
		}
		return core.EnvironmentRuntime{}, fmt.Errorf("Kubernetes Environment namespace %q already exists: %w", ref, core.ErrAlreadyExists)
	}

	if err := p.createManifest(ctx, namespaceManifest(ref, spec.Name)); err != nil {
		return core.EnvironmentRuntime{}, fmt.Errorf("create Kubernetes Environment namespace %q: %w", ref, err)
	}
	cleanup := func(cause error) (core.EnvironmentRuntime, error) {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		if err := p.deleteOwnedNamespace(cleanupCtx, ref, spec.Name); err != nil {
			return core.EnvironmentRuntime{}, errors.Join(cause, err, core.ErrRecoveryRequired)
		}
		return core.EnvironmentRuntime{}, cause
	}

	workload := workloadManifest(ref, spec.Name, workspace, spec.ReadOnly, p.image, p.runtimeClass, resources)
	if err := p.createManifest(ctx, workload); err != nil {
		return cleanup(fmt.Errorf("create Kubernetes Environment workload %q: %w", ref, err))
	}

	waitCtx, cancel := context.WithTimeout(ctx, p.startupTimeout)
	defer cancel()
	if _, err := p.runner.Run(waitCtx, p.kubectl, "-n", ref, "wait", "--for=condition=Ready", "pod/"+podName, "--timeout="+p.startupTimeout.String()); err != nil {
		return cleanup(fmt.Errorf("wait for Kubernetes Environment %q: %w", ref, err))
	}

	initName, err := p.exec(ctx, ref, []string{"cat", "/proc/1/comm"})
	if err != nil {
		return cleanup(fmt.Errorf("verify systemd init in Kubernetes Environment %q: %w", ref, err))
	}
	if strings.TrimSpace(initName.Stdout) != "systemd" {
		return cleanup(fmt.Errorf("Kubernetes Environment %q PID 1 is %q, want systemd: %w", ref, strings.TrimSpace(initName.Stdout), core.ErrIncompatibleState))
	}
	if !spec.ReadOnly {
		if _, err := p.exec(ctx, ref, []string{"test", "-w", "/workspace"}); err != nil {
			return cleanup(errors.Join(fmt.Errorf("workspace %q is not writable in Kubernetes Environment %q: %w", workspace, ref, err), core.ErrUnsupported))
		}
	}

	return core.EnvironmentRuntime{Ref: ref, Resources: resources}, nil
}

func (p *Provider) ExecEnvironment(ctx context.Context, ref string, req core.ExecutionRequest) (core.ExecutionResult, error) {
	if len(req.Argv) == 0 {
		return core.ExecutionResult{}, core.ErrInvalidArgument
	}
	if _, err := p.validateOwnedRef(ctx, ref); err != nil {
		return core.ExecutionResult{}, err
	}
	result, err := p.exec(ctx, ref, req.Argv)
	return core.ExecutionResult{
		ExitCode:        result.ExitCode,
		Stdout:          result.Stdout,
		Stderr:          result.Stderr,
		StdoutTruncated: result.StdoutTruncated,
		StderrTruncated: result.StderrTruncated,
		StdoutBytes:     result.StdoutBytes,
		StderrBytes:     result.StderrBytes,
	}, err
}

func (p *Provider) ShellEnvironment(ctx context.Context, ref string) error {
	if _, err := p.validateOwnedRef(ctx, ref); err != nil {
		return err
	}
	return p.interactive(ctx, ref, podName)
}

func (p *Provider) DeleteEnvironment(ctx context.Context, ref string) error {
	name, err := p.validateOwnedRef(ctx, ref)
	if err != nil {
		return err
	}
	return p.deleteOwnedNamespace(ctx, ref, name)
}

func (p *Provider) InspectEnvironment(ctx context.Context, ref string) (core.EnvironmentRuntimeStatus, error) {
	if _, err := p.validateOwnedRef(ctx, ref); err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	result, err := p.runner.Run(ctx, p.kubectl, "-n", ref, "get", "pod", podName, "--ignore-not-found", "-o", "json")
	if err != nil {
		return core.EnvironmentRuntimeStatus{}, err
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return core.EnvironmentRuntimeStatus{}, core.ErrNotFound
	}
	var pod struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &pod); err != nil {
		return core.EnvironmentRuntimeStatus{}, fmt.Errorf("decode Kubernetes Pod state: %w", core.ErrIncompatibleState)
	}
	switch pod.Status.Phase {
	case "Running":
		return core.EnvironmentRuntimeStatus{State: core.EnvironmentRunning}, nil
	case "Succeeded", "Failed":
		return core.EnvironmentRuntimeStatus{State: core.EnvironmentStopped}, nil
	default:
		return core.EnvironmentRuntimeStatus{State: core.EnvironmentUnknown}, nil
	}
}

func (p *Provider) exec(ctx context.Context, ref string, argv []string) (host.Result, error) {
	args := append([]string{"-n", ref, "exec", podName, "--"}, argv...)
	return p.runner.Run(ctx, p.kubectl, args...)
}

func (p *Provider) execInteractive(ctx context.Context, ref, pod string) error {
	cmd := exec.CommandContext(ctx, p.kubectl, "-n", ref, "exec", "-it", pod, "--", "/bin/bash")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *Provider) validateOwnedRef(ctx context.Context, ref string) (string, error) {
	name, err := environmentNameFromRef(ref)
	if err != nil {
		return "", err
	}
	namespace, err := p.namespace(ctx, ref)
	if err != nil {
		return "", err
	}
	if namespace == nil {
		return "", core.ErrNotFound
	}
	if err := validateOwnedNamespace(namespace, ref, name); err != nil {
		return "", err
	}
	return name, nil
}

func (p *Provider) namespace(ctx context.Context, ref string) (*namespaceState, error) {
	if _, err := environmentNameFromRef(ref); err != nil {
		return nil, err
	}
	result, err := p.runner.Run(ctx, p.kubectl, "get", "namespace", ref, "--ignore-not-found", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("inspect Kubernetes namespace %q: %w", ref, err)
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return nil, nil
	}
	var state namespaceState
	if err := json.Unmarshal([]byte(result.Stdout), &state); err != nil {
		return nil, fmt.Errorf("decode Kubernetes namespace %q: %w", ref, core.ErrIncompatibleState)
	}
	return &state, nil
}

func (p *Provider) deleteOwnedNamespace(ctx context.Context, ref, name string) error {
	state, err := p.namespace(ctx, ref)
	if err != nil {
		return err
	}
	if state == nil {
		return nil
	}
	if err := validateOwnedNamespace(state, ref, name); err != nil {
		return err
	}
	if _, err := p.runner.Run(ctx, p.kubectl, "delete", "namespace", ref, "--wait=true"); err != nil {
		return fmt.Errorf("delete Kubernetes Environment namespace %q: %w", ref, err)
	}
	return nil
}

func (p *Provider) createManifest(ctx context.Context, manifest any) error {
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp("", "hacocoon-kube-*.json")
	if err != nil {
		return err
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	_, err = p.runner.Run(ctx, p.kubectl, "create", "-f", path)
	return err
}

type namespaceState struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
}

func validateOwnedNamespace(state *namespaceState, ref, name string) error {
	if state == nil || state.Metadata.Name != ref || state.Metadata.Labels[managedByLabel] != managedByValue || state.Metadata.Labels[roleLabel] != roleEnvironment || state.Metadata.Labels[environmentLabel] != name {
		return fmt.Errorf("Kubernetes namespace %q is not exactly Hacocoon-owned Environment state: %w", ref, core.ErrIncompatibleState)
	}
	return nil
}

func namespaceManifest(ref, name string) map[string]any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]any{
			"name":   ref,
			"labels": managedLabels(name),
		},
	}
}

func workloadManifest(ref, name, workspace string, readOnly bool, image, runtimeClass string, resources core.ResourceBudget) map[string]any {
	container := map[string]any{
		"name":            podName,
		"image":           image,
		"imagePullPolicy": "IfNotPresent",
		"command":         []string{"/sbin/init"},
		"securityContext": map[string]any{
			"privileged":             false,
			"readOnlyRootFilesystem": false,
		},
		"volumeMounts": []any{
			map[string]any{"name": "workspace", "mountPath": "/workspace", "readOnly": readOnly},
		},
	}
	limits := map[string]string{}
	if resources.CPU.Mode == core.ResourceLimitFinite {
		limits["cpu"] = strconv.FormatUint(resources.CPU.Value, 10)
	}
	if resources.MemoryBytes.Mode == core.ResourceLimitFinite {
		limits["memory"] = strconv.FormatUint(resources.MemoryBytes.Value, 10)
	}
	if resources.RootBytes.Mode == core.ResourceLimitFinite {
		limits["ephemeral-storage"] = strconv.FormatUint(resources.RootBytes.Value, 10)
	}
	if len(limits) != 0 {
		container["resources"] = map[string]any{"limits": limits}
	}

	labels := managedLabels(name)
	labels["app.kubernetes.io/name"] = "hacocoon-environment"
	pod := map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata": map[string]any{
			"name":      podName,
			"namespace": ref,
			"labels":    labels,
		},
		"spec": map[string]any{
			"runtimeClassName":             runtimeClass,
			"hostUsers":                    false,
			"automountServiceAccountToken": false,
			"enableServiceLinks":           false,
			"hostNetwork":                  false,
			"hostPID":                      false,
			"hostIPC":                      false,
			"restartPolicy":                "Never",
			"containers":                   []any{container},
			"volumes": []any{
				map[string]any{
					"name": "workspace",
					"hostPath": map[string]any{
						"path": workspace,
						"type": "Directory",
					},
				},
			},
		},
	}
	networkPolicy := map[string]any{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "NetworkPolicy",
		"metadata": map[string]any{
			"name":      "default-deny",
			"namespace": ref,
			"labels":    managedLabels(name),
		},
		"spec": map[string]any{
			"podSelector": map[string]any{},
			"policyTypes": []string{"Ingress", "Egress"},
		},
	}
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "List",
		"items":      []any{networkPolicy, pod},
	}
}

func managedLabels(name string) map[string]string {
	return map[string]string{
		managedByLabel:   managedByValue,
		roleLabel:        roleEnvironment,
		environmentLabel: name,
	}
}

func validateEnvironmentName(name string) error {
	if name == "host" {
		return fmt.Errorf("environment name %q is reserved for trusted Hacocoon infrastructure: %w", name, core.ErrInvalidArgument)
	}
	if len(name) == 0 || len(name) > 58 || !environmentNameRE.MatchString(name) {
		return fmt.Errorf("environment name %q is not a Kubernetes DNS label compatible Hacocoon name: %w", name, core.ErrInvalidArgument)
	}
	return nil
}

func namespaceFor(name string) string { return "haco-" + name }

func environmentNameFromRef(ref string) (string, error) {
	if !strings.HasPrefix(ref, "haco-") {
		return "", fmt.Errorf("invalid Kubernetes Environment ref %q: %w", ref, core.ErrInvalidArgument)
	}
	name := strings.TrimPrefix(ref, "haco-")
	if err := validateEnvironmentName(name); err != nil {
		return "", err
	}
	if namespaceFor(name) != ref {
		return "", core.ErrInvalidArgument
	}
	return name, nil
}

func validateWorkspacePath(path string) (string, error) {
	if path == "" || strings.ContainsAny(path, "\r\n\x00") || !filepath.IsAbs(path) {
		return "", fmt.Errorf("workspace path must be an absolute path without control characters: %w", core.ErrInvalidArgument)
	}
	clean := filepath.Clean(path)
	if clean != path {
		return "", fmt.Errorf("workspace path %q is not canonical: %w", path, core.ErrInvalidArgument)
	}
	return clean, nil
}

func validDNSSubdomain(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || !environmentNameRE.MatchString(label) {
			return false
		}
	}
	return true
}
