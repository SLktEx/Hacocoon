package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	managedEnvironmentMarkerKey   = "user.hacocoon.kind"
	managedEnvironmentMarkerValue = "environment"
)

type seedHarvestRunner struct {
	next    host.Runner
	project string
}

// WrapSeedHarvestRunner adds a credential-free fast path for Seed acquisition.
// Only exact immutable Seed pulls are intercepted. All other commands are
// forwarded unchanged to the trusted Host runner.
func WrapSeedHarvestRunner(next host.Runner) host.Runner {
	if next == nil {
		return next
	}
	return &seedHarvestRunner{next: next, project: defaultProject}
}

func (r *seedHarvestRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	if r == nil || r.next == nil {
		return host.Result{ExitCode: -1}, core.ErrRuntimeUnavailable
	}
	ref, harvestable := seedPullIdentity(name, args)
	if !harvestable {
		return r.next.Run(ctx, name, args...)
	}

	harvestErr := r.harvest(ctx, ref)
	if harvestErr == nil {
		return host.Result{}, nil
	}
	result, pullErr := r.next.Run(ctx, name, args...)
	if pullErr != nil {
		return result, errors.Join(pullErr, fmt.Errorf("harvest exact Seed image from managed Environment: %w", harvestErr))
	}
	return result, nil
}

func seedPullIdentity(name string, args []string) (string, bool) {
	if name != "nerdctl" || len(args) != 4 || args[0] != "--namespace" || args[1] != seedHostNamespace || args[2] != "pull" {
		return "", false
	}
	ref := args[3]
	if ref == "" || strings.TrimSpace(ref) != ref || strings.HasPrefix(ref, "-") || strings.ContainsAny(ref, "\t\r\n") || strings.Count(ref, "@") != 1 {
		return "", false
	}
	cut := strings.LastIndexByte(ref, '@')
	if cut <= 0 || cut+len("@sha256:") >= len(ref) || !strings.HasPrefix(ref[cut+1:], "sha256:") {
		return "", false
	}
	fingerprint := ref[cut+1+len("sha256:"):]
	if !baseFingerprintPattern.MatchString(fingerprint) {
		return "", false
	}
	return ref, true
}

type seedHarvestInstance struct {
	Name   string            `json:"name"`
	Status string            `json:"status"`
	Config map[string]string `json:"config"`
}

func (r *seedHarvestRunner) harvest(ctx context.Context, ref string) error {
	instances, err := r.harvestCandidates(ctx)
	if err != nil {
		return err
	}
	if len(instances) == 0 {
		return core.ErrNotFound
	}
	var failures []error
	for _, instance := range instances {
		if err := r.harvestFromEnvironment(ctx, instance, ref); err == nil {
			return nil
		} else {
			failures = append(failures, fmt.Errorf("%s: %w", instance, err))
		}
	}
	return errors.Join(failures...)
}

func (r *seedHarvestRunner) harvestCandidates(ctx context.Context) ([]string, error) {
	result, err := r.next.Run(ctx, "incus", "list", "--project", r.project, "--format", "json")
	if err != nil || result.ExitCode != 0 {
		return nil, fmt.Errorf("list managed Environments for Seed harvest: %w", commandResultError(result, err))
	}
	var instances []seedHarvestInstance
	if err := json.Unmarshal([]byte(result.Stdout), &instances); err != nil {
		return nil, fmt.Errorf("decode managed Environment inventory for Seed harvest: %w", core.ErrIncompatibleState)
	}
	candidates := make([]string, 0)
	seen := map[string]struct{}{}
	for _, instance := range instances {
		if !strings.EqualFold(strings.TrimSpace(instance.Status), "running") || instance.Config[managedEnvironmentMarkerKey] != managedEnvironmentMarkerValue {
			continue
		}
		name := strings.TrimSpace(instance.Name)
		if name == "" || name != instance.Name || !strings.HasPrefix(name, "haco-") || strings.HasPrefix(name, "-") || strings.ContainsAny(name, "\t\r\n") || seedBuilderNamePattern.MatchString(name) {
			return nil, fmt.Errorf("managed Environment inventory contains invalid harvest source %q: %w", instance.Name, core.ErrIncompatibleState)
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("managed Environment inventory repeats harvest source %q: %w", name, core.ErrIncompatibleState)
		}
		seen[name] = struct{}{}
		candidates = append(candidates, name)
	}
	sort.Strings(candidates)
	return candidates, nil
}

func (r *seedHarvestRunner) harvestFromEnvironment(ctx context.Context, environment, ref string) error {
	token, err := randomHarvestToken()
	if err != nil {
		return err
	}
	guestArchive := "/tmp/haco-seed-harvest-" + token + ".tar"
	dir, err := os.MkdirTemp("", "haco-seed-harvest-*")
	if err != nil {
		return fmt.Errorf("create trusted Seed harvest directory: %w", err)
	}
	defer os.RemoveAll(dir)
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	hostArchive := filepath.Join(dir, "image.tar")

	saveArgs := []string{"exec", environment, "--project", r.project, "--", "nerdctl", "save", "-o", guestArchive, ref}
	result, err := r.next.Run(ctx, "incus", saveArgs...)
	if err != nil || result.ExitCode != 0 {
		r.cleanupGuestArchive(environment, guestArchive)
		return fmt.Errorf("save exact OCI content inside Environment: %w", commandResultError(result, err))
	}

	pullResult, pullErr := r.next.Run(ctx, "incus", "file", "pull", environment+guestArchive, hostArchive, "--project", r.project)
	r.cleanupGuestArchive(environment, guestArchive)
	if pullErr != nil || pullResult.ExitCode != 0 {
		return fmt.Errorf("copy OCI archive from managed Environment: %w", commandResultError(pullResult, pullErr))
	}
	info, err := os.Lstat(hostArchive)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("managed Environment Seed harvest produced invalid archive: %w", core.ErrIncompatibleState)
	}

	loadResult, loadErr := r.next.Run(ctx, "nerdctl", "--namespace", seedHostNamespace, "load", "-i", hostArchive)
	if loadErr != nil || loadResult.ExitCode != 0 {
		return fmt.Errorf("load harvested OCI archive into trusted Host cache: %w", commandResultError(loadResult, loadErr))
	}
	inspectResult, inspectErr := r.next.Run(ctx, "nerdctl", "--namespace", seedHostNamespace, "image", "inspect", ref)
	if inspectErr != nil || inspectResult.ExitCode != 0 {
		return fmt.Errorf("verify harvested immutable OCI identity on trusted Host: %w", commandResultError(inspectResult, inspectErr))
	}
	return nil
}

func (r *seedHarvestRunner) cleanupGuestArchive(environment, path string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = r.next.Run(ctx, "incus", "exec", environment, "--project", r.project, "--", "rm", "-f", path)
}

func randomHarvestToken() (string, error) {
	var raw [6]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate Seed harvest token: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}
