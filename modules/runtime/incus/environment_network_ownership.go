package incus

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	environmentNetworkOwnerKey   = "user.hacocoon.owner"
	environmentNetworkOwnerValue = "environment-network-v1"
)

// WrapEnvironmentNetworkOwnershipRunner makes Environment bridge ownership a
// property of the Incus command boundary rather than a convention remembered
// by individual network call sites.
//
// Dedicated Hacocoon Environment bridges are marked at creation. Any later
// attachment or deletion through the production runtime must prove the marker
// first. A same-named unmanaged/adversarial bridge therefore fails closed
// instead of being silently adopted or destroyed.
func WrapEnvironmentNetworkOwnershipRunner(inner host.Runner) host.Runner {
	if inner == nil {
		return nil
	}
	return environmentNetworkOwnershipRunner{inner: inner}
}

type environmentNetworkOwnershipRunner struct {
	inner host.Runner
}

func (r environmentNetworkOwnershipRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	if strings.ToLower(filepath.Base(name)) != "incus" {
		return r.inner.Run(ctx, name, args...)
	}

	if bridge, ok := environmentNetworkCreate(args); ok {
		if project := incusProjectArg(args); project != "" && project != sandboxBridgeResourceProject {
			return ownershipFailure(fmt.Sprintf("refusing Environment bridge %s in Incus project %q", bridge, project))
		}
		args = insertIncusConfigBeforeFlags(args, environmentNetworkOwnerKey+"="+environmentNetworkOwnerValue)
		return r.inner.Run(ctx, name, args...)
	}

	if bridge, ok := environmentNetworkDelete(args); ok {
		if err := r.requireEnvironmentNetworkOwnership(ctx, bridge); err != nil {
			return ownershipFailure(err.Error())
		}
		return r.inner.Run(ctx, name, args...)
	}

	if bridge, ok := environmentNetworkAttachment(args); ok {
		if err := r.requireEnvironmentNetworkOwnership(ctx, bridge); err != nil {
			return ownershipFailure(err.Error())
		}
	}
	return r.inner.Run(ctx, name, args...)
}

func (r environmentNetworkOwnershipRunner) requireEnvironmentNetworkOwnership(ctx context.Context, bridge string) error {
	result, err := r.inner.Run(ctx, "incus", "network", "get", bridge, environmentNetworkOwnerKey, "--project", sandboxBridgeResourceProject)
	if err != nil {
		return fmt.Errorf("prove ownership of Environment bridge %s: %w", bridge, err)
	}
	owner := strings.TrimSpace(result.Stdout)
	if owner != environmentNetworkOwnerValue {
		return fmt.Errorf("Environment bridge %s ownership marker=%q, want %q: %w", bridge, owner, environmentNetworkOwnerValue, core.ErrIncompatibleState)
	}
	return nil
}

func ownershipFailure(message string) (host.Result, error) {
	err := errors.Join(errors.New(message), core.ErrIncompatibleState)
	return host.Result{ExitCode: -1, Stderr: err.Error()}, err
}

func environmentNetworkCreate(args []string) (string, bool) {
	if len(args) < 3 || args[0] != "network" || args[1] != "create" || !isEnvironmentBridge(args[2]) {
		return "", false
	}
	return args[2], true
}

func environmentNetworkDelete(args []string) (string, bool) {
	if len(args) < 3 || args[0] != "network" || args[1] != "delete" || !isEnvironmentBridge(args[2]) {
		return "", false
	}
	return args[2], true
}

func environmentNetworkAttachment(args []string) (string, bool) {
	if len(args) < 5 || args[0] != "config" || args[1] != "device" {
		return "", false
	}
	switch args[2] {
	case "add", "override", "set":
	default:
		return "", false
	}
	for _, arg := range args[3:] {
		if !strings.HasPrefix(arg, "network=") {
			continue
		}
		bridge := strings.TrimPrefix(arg, "network=")
		if isEnvironmentBridge(bridge) {
			return bridge, true
		}
	}
	return "", false
}

func isEnvironmentBridge(name string) bool {
	return strings.HasPrefix(name, sandboxRoutedHostPrefix) && len(name) > len(sandboxRoutedHostPrefix)
}

func incusProjectArg(args []string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--project" {
			return args[i+1]
		}
	}
	return ""
}

func insertIncusConfigBeforeFlags(args []string, value string) []string {
	for i := 3; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			out := make([]string, 0, len(args)+1)
			out = append(out, args[:i]...)
			out = append(out, value)
			out = append(out, args[i:]...)
			return out
		}
	}
	return append(append([]string(nil), args...), value)
}
