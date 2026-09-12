//go:build linux

package incus

import (
	"context"
	"errors"
	"github.com/SLktEx/Hacocoon/internal/core"
	"io"
	"sort"
)

// CreateEnvironmentFromArchive uses the owned native transport image, then the
// current sandbox configuration. Canonical lifecycle owns the instance receipt
// and failed-instance cleanup; the transport adapter owns image cleanup.
func (p *SandboxProvider) CreateEnvironmentFromArchive(ctx context.Context, spec core.EnvironmentRuntimeSpec, source io.ReadSeeker, privateRoot string, limit int64, record func(core.EnvironmentRuntime) error) (created core.EnvironmentRuntime, err error) {
	if p == nil || p.BaseProvider == nil || p.Runtime == nil || record == nil || !core.ValidEnvironmentInstanceID(spec.InstanceID) || spec.WorkspacePath == "" || spec.TemporaryWorkspace || spec.ResourceMaintenance || spec.Base != "" {
		return created, core.ErrInvalidArgument
	}
	if err = validateManagedInstanceRef("haco-" + spec.Name); err != nil {
		return created, err
	}
	if err = p.ensureProject(ctx); err != nil {
		return created, err
	}
	err = p.WithImportedRootfs(ctx, source, privateRoot, limit, func(fingerprint string) error {
		var createErr error
		created, createErr = p.createEnvironmentFromImportedImage(ctx, spec, fingerprint, record)
		return createErr
	})
	return created, err
}

func (p *SandboxProvider) createEnvironmentFromImportedImage(ctx context.Context, spec core.EnvironmentRuntimeSpec, fingerprint string, record func(core.EnvironmentRuntime) error) (core.EnvironmentRuntime, error) {
	if !baseFingerprintPattern.MatchString(fingerprint) {
		return core.EnvironmentRuntime{}, core.ErrInvalidArgument
	}
	resources, err := core.ResolveResourceBudget(spec.Resources)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	rootPool, err := p.defaultRootPool(ctx)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	if err := p.ensureRoutedSandboxHost(ctx); err != nil {
		return core.EnvironmentRuntime{}, err
	}
	config, err := p.sandboxProfileConfig(ctx)
	if err != nil {
		return core.EnvironmentRuntime{}, err
	}
	config[environmentInstanceKey] = spec.InstanceID
	config[managedEnvironmentMarkerKey] = managedEnvironmentMarkerValue
	config["boot.autostart"] = "false"
	config["security.privileged"] = "false"
	config["security.nesting"] = "false"
	ref := "haco-" + spec.Name
	args := []string{"init", "local:" + fingerprint, ref, "--project", p.project, "--no-profiles", "--storage", rootPool}
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--config", key+"="+config[key])
	}
	out, err := p.runner.Run(ctx, "incus", args...)
	if err != nil || out.ExitCode != 0 || out.StdoutTruncated {
		return core.EnvironmentRuntime{}, errors.Join(core.ErrRecoveryRequired, err)
	}
	created := core.EnvironmentRuntime{Ref: ref, Resources: resources}
	// No fallible provider call between native creation and durable ownership.
	if err := record(created); err != nil {
		return created, err
	}
	if err := p.configureSandboxEnvironment(ctx, ref, spec, resources, false); err != nil {
		return created, err
	}
	if err := p.renewGuestSSHIdentity(ctx, ref); err != nil {
		return created, err
	}
	return created, nil
}
