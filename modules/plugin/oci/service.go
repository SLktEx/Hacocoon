package oci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

type Driver string

const (
	DriverNerdctl Driver = "nerdctl"
	DriverDocker  Driver = "docker"
)

func ParseDriver(value string) (Driver, error) {
	switch Driver(strings.ToLower(strings.TrimSpace(value))) {
	case DriverNerdctl:
		return DriverNerdctl, nil
	case DriverDocker:
		return DriverDocker, nil
	default:
		return "", fmt.Errorf("unsupported OCI plugin driver %q: %w", value, core.ErrInvalidArgument)
	}
}

type environmentExecutor interface {
	ExecEnvironment(context.Context, string, core.ExecutionRequest) (core.ExecutionResult, error)
}

type Service struct {
	runtime              environmentExecutor
	environmentStatePath string
	driver               Driver
}

func New(runtime environmentExecutor, environmentStatePath string, driver Driver) (*Service, error) {
	if runtime == nil || strings.TrimSpace(environmentStatePath) == "" {
		return nil, core.ErrInvalidArgument
	}
	if driver != DriverNerdctl && driver != DriverDocker {
		return nil, core.ErrInvalidArgument
	}
	return &Service{runtime: runtime, environmentStatePath: environmentStatePath, driver: driver}, nil
}

func (s *Service) Driver() Driver {
	if s == nil {
		return ""
	}
	return s.driver
}

func readEnvironments(path string) ([]core.Environment, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []core.Environment{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read Environment state for OCI plugin: %w", err)
	}
	var state struct {
		Environments map[string]core.Environment `json:"environments"`
	}
	if err := json.Unmarshal(contents, &state); err != nil {
		return nil, fmt.Errorf("decode Environment state for OCI plugin: %w", err)
	}
	result := make([]core.Environment, 0, len(state.Environments))
	for name, environment := range state.Environments {
		if environment.Name == "" {
			environment.Name = name
		}
		if strings.TrimSpace(environment.Name) == "" || strings.TrimSpace(environment.RuntimeRef) == "" {
			return nil, fmt.Errorf("invalid Environment metadata for OCI plugin: %w", core.ErrIncompatibleState)
		}
		result = append(result, environment)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func commandFailure(action, stderr string, err error, exitCode int) error {
	reason := strings.TrimSpace(stderr)
	if reason == "" && err != nil {
		reason = err.Error()
	}
	if reason == "" {
		reason = fmt.Sprintf("exit code %d", exitCode)
	}
	return fmt.Errorf("%s: %s: %w", action, reason, core.ErrRuntimeUnavailable)
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
