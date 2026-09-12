package incus

import (
	"context"
	"strings"

	"github.com/SLktEx/Hacocoon/internal/core"
)

const environmentInstanceKey = "user.hacocoon.instance-id"

// Direct stateless provider callers may omit the identity. Such instances cannot
// be adopted as snapshot sources. Stateful creation always supplies its lease ID.
func environmentIdentityArgs(instance string) ([]string, error) {
	if instance == "" {
		return nil, nil
	}
	if !core.ValidEnvironmentInstanceID(instance) {
		return nil, core.ErrInvalidArgument
	}
	return []string{"--config", environmentInstanceKey + "=" + instance}, nil
}

// VerifyEnvironmentIdentity never adopts or repairs missing provider ownership.
func (r *Runtime) VerifyEnvironmentIdentity(ctx context.Context, ref, instance string) error {
	if !core.ValidEnvironmentInstanceID(instance) || ref == trustedHostName {
		return core.ErrInvalidArgument
	}
	if err := validateManagedInstanceRef(ref); err != nil {
		return err
	}
	result, err := r.runner.Run(ctx, "incus", "config", "get", ref, environmentInstanceKey, "--project", r.project)
	if err != nil || result.StdoutTruncated {
		return core.ErrRuntimeUnavailable
	}
	if strings.TrimSpace(result.Stdout) != instance {
		return core.ErrCapabilityStale
	}
	return nil
}
