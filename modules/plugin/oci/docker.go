package oci

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

//go:embed packaging/systemd/hacocoon-docker.socket
var hacocoonDockerSocketUnit string

//go:embed packaging/systemd/hacocoon-docker.service
var hacocoonDockerServiceUnit string

type DockerCompatibilityStatus struct {
	Environment         string `json:"environment"`
	DockerCLI           bool   `json:"docker_cli"`
	DockerDaemon        bool   `json:"docker_daemon"`
	Containerd          bool   `json:"containerd"`
	Systemd             bool   `json:"systemd"`
	DockerGroup         bool   `json:"docker_group"`
	SocketUnitVerified  bool   `json:"socket_unit_verified"`
	ServiceUnitVerified bool   `json:"service_unit_verified"`
	SocketEnabled       bool   `json:"socket_enabled"`
	SocketActive        bool   `json:"socket_active"`
	EngineActive        bool   `json:"engine_active"`
	ContainerdActive    bool   `json:"containerd_active"`
	VendorDockerEnabled bool   `json:"vendor_docker_enabled"`
	VendorDockerActive  bool   `json:"vendor_docker_active"`
	Ready               bool   `json:"ready"`
}

func (s *Service) DockerStatus(ctx context.Context, environment string) (DockerCompatibilityStatus, error) {
	if err := s.requireDockerDriver(); err != nil {
		return DockerCompatibilityStatus{}, err
	}
	ref, err := s.environmentRuntimeRef(environment)
	if err != nil {
		return DockerCompatibilityStatus{}, err
	}
	return s.dockerStatusByRef(ctx, environment, ref)
}

func (s *Service) PrepareDocker(ctx context.Context, environment string) (DockerCompatibilityStatus, error) {
	if err := s.requireDockerDriver(); err != nil {
		return DockerCompatibilityStatus{}, err
	}
	ref, err := s.environmentRuntimeRef(environment)
	if err != nil {
		return DockerCompatibilityStatus{}, err
	}

	before, err := s.dockerStatusByRef(ctx, environment, ref)
	if err != nil {
		return before, err
	}
	if err := validateDockerCompatibilityPrerequisites(before); err != nil {
		return before, err
	}
	if before.VendorDockerActive {
		return before, fmt.Errorf("Environment %q has an active vendor Docker service/socket; refusing to stop an existing daemon automatically: %w", environment, core.ErrRuntimeUnavailable)
	}
	if before.Ready {
		return before, nil
	}

	result, execErr := s.runtime.ExecEnvironment(ctx, ref, core.ExecutionRequest{Argv: []string{"/bin/sh", "-c", dockerPrepareScript}})
	if execErr != nil || result.ExitCode != 0 {
		return before, commandFailure("prepare Environment-local Docker compatibility", result.Stderr, execErr, result.ExitCode)
	}

	after, err := s.dockerStatusByRef(ctx, environment, ref)
	if err != nil {
		return after, err
	}
	if err := validateDockerCompatibilityPrerequisites(after); err != nil {
		return after, err
	}
	if !after.Ready {
		return after, fmt.Errorf("Environment-local Docker socket activation did not reach the expected state: %w", core.ErrRuntimeUnavailable)
	}
	return after, nil
}

func (s *Service) requireDockerDriver() error {
	if s == nil || s.runtime == nil {
		return core.ErrInvalidArgument
	}
	if s.driver != DriverDocker {
		return fmt.Errorf("Docker compatibility requires HACO_PLUGIN_OCI=docker: %w", core.ErrRuntimeUnavailable)
	}
	return nil
}

func (s *Service) environmentRuntimeRef(environment string) (string, error) {
	if strings.TrimSpace(environment) == "" || strings.TrimSpace(environment) != environment || hasControl(environment) {
		return "", core.ErrInvalidArgument
	}
	environments, err := readEnvironments(s.environmentStatePath)
	if err != nil {
		return "", err
	}
	for _, candidate := range environments {
		if candidate.Name == environment {
			return candidate.RuntimeRef, nil
		}
	}
	return "", fmt.Errorf("Environment %q: %w", environment, core.ErrNotFound)
}

func (s *Service) dockerStatusByRef(ctx context.Context, environment, ref string) (DockerCompatibilityStatus, error) {
	result, execErr := s.runtime.ExecEnvironment(ctx, ref, core.ExecutionRequest{Argv: []string{"/bin/sh", "-c", dockerProbeScript}})
	if execErr != nil || result.ExitCode != 0 {
		return DockerCompatibilityStatus{Environment: environment}, commandFailure("inspect Environment-local Docker compatibility", result.Stderr, execErr, result.ExitCode)
	}
	values, err := parseDockerProbe(result.Stdout)
	if err != nil {
		return DockerCompatibilityStatus{Environment: environment}, err
	}
	status := DockerCompatibilityStatus{
		Environment:         environment,
		DockerCLI:           values["docker_cli"] == "1",
		DockerDaemon:        values["dockerd"] == "1",
		Containerd:          values["containerd"] == "1",
		Systemd:             values["systemctl"] == "1",
		DockerGroup:         values["docker_group"] == "1",
		SocketUnitVerified:  values["socket_unit_sha256"] == dockerSocketUnitDigest(),
		ServiceUnitVerified: values["service_unit_sha256"] == dockerServiceUnitDigest(),
		SocketEnabled:       values["socket_enabled"] == "1",
		SocketActive:        values["socket_active"] == "1",
		EngineActive:        values["engine_active"] == "1",
		ContainerdActive:    values["containerd_active"] == "1",
		VendorDockerEnabled: values["vendor_docker_enabled"] == "1",
		VendorDockerActive:  values["vendor_docker_active"] == "1",
	}
	status.Ready = status.DockerCLI && status.DockerDaemon && status.Containerd && status.Systemd && status.DockerGroup &&
		status.SocketUnitVerified && status.ServiceUnitVerified && status.SocketEnabled && status.SocketActive &&
		!status.VendorDockerEnabled && !status.VendorDockerActive
	return status, nil
}

func validateDockerCompatibilityPrerequisites(status DockerCompatibilityStatus) error {
	if !status.DockerCLI || !status.DockerDaemon || !status.Containerd || !status.Systemd || !status.DockerGroup {
		return fmt.Errorf("Environment %q does not contain the complete Docker compatibility profile (docker, dockerd, containerd, systemd, docker group): %w", status.Environment, core.ErrRuntimeUnavailable)
	}
	if !status.SocketUnitVerified || !status.ServiceUnitVerified {
		return fmt.Errorf("Environment %q Docker compatibility systemd units are missing or differ from the Hacocoon-pinned units: %w", status.Environment, core.ErrIncompatibleState)
	}
	return nil
}

func parseDockerProbe(output string) (map[string]string, error) {
	allowed := map[string]struct{}{
		"docker_cli": {}, "dockerd": {}, "containerd": {}, "systemctl": {}, "docker_group": {},
		"socket_unit_sha256": {}, "service_unit_sha256": {}, "socket_enabled": {}, "socket_active": {},
		"engine_active": {}, "containerd_active": {}, "vendor_docker_enabled": {}, "vendor_docker_active": {},
	}
	values := make(map[string]string, len(allowed))
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("malformed Docker compatibility probe output: %w", core.ErrIncompatibleState)
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if _, ok := allowed[key]; !ok {
			return nil, fmt.Errorf("unexpected Docker compatibility probe key %q: %w", key, core.ErrIncompatibleState)
		}
		if _, duplicate := values[key]; duplicate {
			return nil, fmt.Errorf("duplicate Docker compatibility probe key %q: %w", key, core.ErrIncompatibleState)
		}
		values[key] = value
	}
	for key := range allowed {
		if _, ok := values[key]; !ok {
			return nil, fmt.Errorf("missing Docker compatibility probe key %q: %w", key, core.ErrIncompatibleState)
		}
	}
	for _, key := range []string{"docker_cli", "dockerd", "containerd", "systemctl", "docker_group", "socket_enabled", "socket_active", "engine_active", "containerd_active", "vendor_docker_enabled", "vendor_docker_active"} {
		if values[key] != "0" && values[key] != "1" {
			return nil, fmt.Errorf("invalid Docker compatibility probe value for %q: %w", key, core.ErrIncompatibleState)
		}
	}
	return values, nil
}

func dockerSocketUnitDigest() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(hacocoonDockerSocketUnit)))
}

func dockerServiceUnitDigest() string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(hacocoonDockerServiceUnit)))
}

const dockerProbeScript = `
has_command() {
    if command -v "$1" >/dev/null 2>&1; then printf '1'; else printf '0'; fi
}
has_group() {
    if getent group "$1" >/dev/null 2>&1; then printf '1'; else printf '0'; fi
}
is_enabled() {
    if systemctl is-enabled "$1" >/dev/null 2>&1; then printf '1'; else printf '0'; fi
}
is_active() {
    if systemctl is-active "$1" >/dev/null 2>&1; then printf '1'; else printf '0'; fi
}
unit_digest() {
    path="$(systemctl show --property=FragmentPath --value "$1" 2>/dev/null || true)"
    if [ -n "$path" ] && [ -f "$path" ]; then
        sha256sum -- "$path" 2>/dev/null | awk '{print $1}'
    else
        printf 'absent'
    fi
}
any_enabled() {
    if systemctl is-enabled "$1" >/dev/null 2>&1 || systemctl is-enabled "$2" >/dev/null 2>&1; then printf '1'; else printf '0'; fi
}
any_active() {
    if systemctl is-active "$1" >/dev/null 2>&1 || systemctl is-active "$2" >/dev/null 2>&1; then printf '1'; else printf '0'; fi
}
printf 'docker_cli\t%s\n' "$(has_command docker)"
printf 'dockerd\t%s\n' "$(has_command dockerd)"
printf 'containerd\t%s\n' "$(has_command containerd)"
printf 'systemctl\t%s\n' "$(has_command systemctl)"
printf 'docker_group\t%s\n' "$(has_group docker)"
printf 'socket_unit_sha256\t%s\n' "$(unit_digest hacocoon-docker.socket)"
printf 'service_unit_sha256\t%s\n' "$(unit_digest hacocoon-docker.service)"
printf 'socket_enabled\t%s\n' "$(is_enabled hacocoon-docker.socket)"
printf 'socket_active\t%s\n' "$(is_active hacocoon-docker.socket)"
printf 'engine_active\t%s\n' "$(is_active hacocoon-docker.service)"
printf 'containerd_active\t%s\n' "$(is_active containerd.service)"
printf 'vendor_docker_enabled\t%s\n' "$(any_enabled docker.service docker.socket)"
printf 'vendor_docker_active\t%s\n' "$(any_active docker.service docker.socket)"
`

const dockerPrepareScript = `
set -eu
# Do not stop a pre-existing Docker daemon here. PrepareDocker checks for an
# active vendor unit first and fails closed rather than disrupting guest work.
systemctl disable docker.service docker.socket >/dev/null 2>&1 || true
systemctl enable hacocoon-docker.socket >/dev/null
systemctl start hacocoon-docker.socket
`
