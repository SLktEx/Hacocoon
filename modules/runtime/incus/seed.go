package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
	"github.com/SLktEx/Hacocoon/internal/seedbuild"
)

const seedHostNamespace = "hacocoon-seed"

var nestedOCIConfig = map[string]string{
	"security.nesting":                     "true",
	"security.syscalls.intercept.mknod":    "true",
	"security.syscalls.intercept.setxattr": "true",
}

const hacocoonDockerSocketUnit = `[Unit]
Description=Hacocoon Docker Engine API compatibility socket
Documentation=https://docs.docker.com/engine/

[Socket]
ListenStream=/run/docker.sock
SocketMode=0660
SocketUser=root
SocketGroup=docker
RemoveOnStop=true
Service=hacocoon-docker.service

[Install]
WantedBy=sockets.target
`

const hacocoonDockerServiceUnit = `[Unit]
Description=Hacocoon Docker Engine compatibility daemon
Documentation=https://docs.docker.com/engine/
Requires=hacocoon-docker.socket containerd.service
After=hacocoon-docker.socket containerd.service network-online.target
Wants=network-online.target

[Service]
Type=notify
ExecStart=/usr/bin/dockerd -H fd:// --containerd=/run/containerd/containerd.sock
ExecReload=/bin/kill -s HUP $MAINPID
TimeoutStartSec=0
Restart=on-failure
RestartSec=2
Delegate=yes
KillMode=process
OOMScoreAdjust=-500
`

func (p *SandboxProvider) ResolveParentBase(ctx context.Context, name core.BaseName) (core.BaseRef, error) {
	if p == nil || p.BaseProvider == nil {
		return core.BaseRef{}, core.ErrRuntimeUnavailable
	}
	resolved, err := p.resolveParentBase(ctx, name)
	if err != nil {
		return core.BaseRef{}, err
	}
	return resolved.ref, nil
}

func (p *SandboxProvider) BuildToolingBase(ctx context.Context, parent core.BaseRef) (seedbuild.BuildResult, error) {
	if p == nil || p.BaseProvider == nil || p.Runtime == nil {
		return seedbuild.BuildResult{}, core.ErrRuntimeUnavailable
	}
	resolved, err := p.resolveParentBase(ctx, parent.Name)
	if err != nil {
		return seedbuild.BuildResult{}, err
	}
	if resolved.ref != parent {
		return seedbuild.BuildResult{}, fmt.Errorf("Base %q moved from %s to %s before tooling build: %w", parent.Name, parent.Revision, resolved.ref.Revision, core.ErrIncompatibleState)
	}
	if err := p.ensureProject(ctx); err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("ensure Incus project for tooling build: %w", err)
	}
	rootPool, err := p.defaultRootPool(ctx)
	if err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("resolve tooling builder storage: %w", err)
	}

	nerdctlBinary, err := seedNerdctlBinary()
	if err != nil {
		return seedbuild.BuildResult{}, err
	}
	provisionDir, err := os.MkdirTemp("", "haco-tooling-provision-*")
	if err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("create tooling provision directory: %w", err)
	}
	defer os.RemoveAll(provisionDir)
	if err := os.Chmod(provisionDir, 0o700); err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("secure tooling provision directory: %w", err)
	}
	files, err := writeToolingProvisionFiles(provisionDir)
	if err != nil {
		return seedbuild.BuildResult{}, err
	}

	toolingNetwork, cleanupToolingNetwork, err := p.createToolingBuilderNetwork(ctx)
	if err != nil {
		return seedbuild.BuildResult{}, err
	}
	toolingNetworkCleanupNeeded := true
	defer func() {
		if toolingNetworkCleanupNeeded {
			_ = cleanupToolingNetwork(nil)
		}
	}()

	builder, err := newSeedBuilderName("tooling")
	if err != nil {
		return seedbuild.BuildResult{}, err
	}
	if _, err := p.runner.Run(ctx, "incus", "init", resolved.pinnedSource, builder,
		"--project", p.project,
		"--no-profiles",
		"--storage", rootPool,
	); err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("init tooling Base builder: %w", err)
	}
	cleanup := p.seedBuilderCleanup(builder)
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			_ = cleanup(nil)
		}
	}()

	if err := p.configureNestedOCIInstance(ctx, builder); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if _, err := p.runner.Run(ctx, "incus", "config", "device", "add", builder, "eth0", "nic",
		"network="+toolingNetwork,
		"name=eth0",
		"--project", p.project,
	); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("attach tooling builder network: %w", err))
	}
	if _, err := p.runner.Run(ctx, "incus", "start", builder, "--project", p.project); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("start tooling Base builder: %w", err))
	}

	if err := p.pushBuilderFile(ctx, builder, files.policyRC, "/usr/sbin/policy-rc.d", 0o755); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.guestExec(ctx, builder, "apt-get", "update"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("update tooling Base package metadata: %w", err))
	}
	if err := p.guestExec(ctx, builder, "env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y",
		"ca-certificates", "containerd", "containernetworking-plugins", "docker.io", "openssh-server",
	); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("install tooling Base OCI packages: %w", err))
	}
	if err := p.guestExec(ctx, builder, "rm", "-f", "/usr/sbin/policy-rc.d"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.pushBuilderFile(ctx, builder, nerdctlBinary, "/usr/local/bin/nerdctl", 0o755); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.pushBuilderFile(ctx, builder, files.socketUnit, "/etc/systemd/system/hacocoon-docker.socket", 0o644); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.pushBuilderFile(ctx, builder, files.serviceUnit, "/etc/systemd/system/hacocoon-docker.service", 0o644); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.guestExec(ctx, builder, "systemctl", "daemon-reload"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.guestExec(ctx, builder, "systemctl", "disable", "--now", "docker.service", "docker.socket"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("disable vendor Docker units: %w", err))
	}
	if err := p.guestExec(ctx, builder, "systemctl", "enable", "containerd.service", "hacocoon-docker.socket"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("enable Hacocoon OCI services: %w", err))
	}
	if err := p.guestExec(ctx, builder, "systemctl", "restart", "containerd.service"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("start containerd in tooling Base: %w", err))
	}
	if err := p.guestExec(ctx, builder, "systemctl", "start", "hacocoon-docker.socket"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("start Hacocoon Docker compatibility socket: %w", err))
	}
	if err := p.expectGuestUnitState(ctx, builder, "hacocoon-docker.socket", "active"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.expectGuestUnitState(ctx, builder, "hacocoon-docker.service", "inactive"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("Docker compatibility service started before socket use: %w", err))
	}
	for _, command := range [][]string{{"nerdctl", "--version"}, {"docker", "--version"}} {
		if err := p.guestExec(ctx, builder, command...); err != nil {
			return seedbuild.BuildResult{}, cleanup(fmt.Errorf("verify tooling command %q: %w", command[0], err))
		}
	}
	// Exercise the real Docker Engine API once while the trusted tooling builder
	// is networked. This proves socket activation reaches the instance-local
	// dockerd/containerd pair instead of publishing a CLI-only compatibility
	// image that fails later in an Environment.
	if err := p.guestExec(ctx, builder, "docker", "info", "--format", "{{.ServerVersion}}"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("verify Docker Engine socket activation in tooling Base: %w", err))
	}
	if err := p.expectGuestUnitState(ctx, builder, "containerd.service", "active"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.expectGuestUnitState(ctx, builder, "hacocoon-docker.service", "active"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	for unit, want := range map[string]string{
		"containerd.service":     "enabled",
		"docker.service":         "disabled",
		"docker.socket":          "disabled",
		"hacocoon-docker.socket": "enabled",
	} {
		if err := p.expectGuestUnitFileState(ctx, builder, unit, want); err != nil {
			return seedbuild.BuildResult{}, cleanup(err)
		}
	}

	// The networked phase is only for public tooling acquisition. Remove the NIC
	// and its short-lived trusted bridge before publishing so the resulting image
	// does not encode any builder network. Seed construction itself starts from
	// this image with no NIC.
	if _, err := p.runner.Run(ctx, "incus", "config", "device", "remove", builder, "eth0", "--project", p.project); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("remove tooling builder network: %w", err))
	}
	if err := cleanupToolingNetwork(nil); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	toolingNetworkCleanupNeeded = false
	if err := p.guestExec(ctx, builder, "systemctl", "stop", "hacocoon-docker.service", "hacocoon-docker.socket", "containerd.service"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("quiesce tooling Base services: %w", err))
	}
	if _, err := p.runner.Run(ctx, "incus", "stop", builder, "--project", p.project); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("stop tooling Base builder: %w", err))
	}
	alias := publishedSeedAlias("tooling", parent.Name)
	published, err := p.publishStoppedBuilder(ctx, builder, alias)
	if err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := cleanup(nil); err != nil {
		return seedbuild.BuildResult{}, errors.Join(err, core.ErrRecoveryRequired)
	}
	cleanupNeeded = false
	return published, nil
}

func (p *SandboxProvider) BuildSeed(ctx context.Context, plan seedbuild.BuildPlan) (seedbuild.BuildResult, error) {
	if p == nil || p.BaseProvider == nil || p.Runtime == nil || plan.Parent.Name == "" || plan.ToolingRevision == "" {
		return seedbuild.BuildResult{}, core.ErrInvalidArgument
	}
	parent, err := p.resolveParentBase(ctx, plan.Parent.Name)
	if err != nil {
		return seedbuild.BuildResult{}, err
	}
	if parent.ref != plan.Parent {
		return seedbuild.BuildResult{}, fmt.Errorf("Base %q moved before Seed build: %w", plan.Parent.Name, core.ErrIncompatibleState)
	}
	toolingFingerprint, err := seedFingerprint(plan.ToolingRevision)
	if err != nil {
		return seedbuild.BuildResult{}, err
	}
	if err := p.verifyLocalImage(ctx, toolingFingerprint); err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("verify tooling Base revision: %w", err)
	}
	if err := p.ensureProject(ctx); err != nil {
		return seedbuild.BuildResult{}, err
	}
	rootPool, err := p.defaultRootPool(ctx)
	if err != nil {
		return seedbuild.BuildResult{}, err
	}

	archive, cleanupArchive, err := p.exportSeedImages(ctx, plan.Images)
	if err != nil {
		return seedbuild.BuildResult{}, err
	}
	defer cleanupArchive()

	builder, err := newSeedBuilderName("seed")
	if err != nil {
		return seedbuild.BuildResult{}, err
	}
	if _, err := p.runner.Run(ctx, "incus", "init", "local:"+toolingFingerprint, builder,
		"--project", p.project,
		"--no-profiles",
		"--storage", rootPool,
	); err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("init offline Seed builder: %w", err)
	}
	cleanup := p.seedBuilderCleanup(builder)
	cleanupNeeded := true
	defer func() {
		if cleanupNeeded {
			_ = cleanup(nil)
		}
	}()
	if err := p.configureNestedOCIInstance(ctx, builder); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if _, err := p.runner.Run(ctx, "incus", "start", builder, "--project", p.project); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("start offline Seed builder: %w", err))
	}
	if err := p.verifyBuilderHasNoNIC(ctx, builder); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.expectGuestUnitState(ctx, builder, "containerd.service", "active"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if archive != "" {
		if err := p.guestExec(ctx, builder, "mkdir", "-p", "/var/lib/hacocoon"); err != nil {
			return seedbuild.BuildResult{}, cleanup(err)
		}
		if err := p.pushBuilderFile(ctx, builder, archive, "/var/lib/hacocoon/seed-images.tar", 0o600); err != nil {
			return seedbuild.BuildResult{}, cleanup(err)
		}
		if err := p.guestExec(ctx, builder, "nerdctl", "load", "-i", "/var/lib/hacocoon/seed-images.tar"); err != nil {
			return seedbuild.BuildResult{}, cleanup(fmt.Errorf("import OCI content into offline Seed builder: %w", err))
		}
		if err := p.guestExec(ctx, builder, "rm", "-f", "/var/lib/hacocoon/seed-images.tar"); err != nil {
			return seedbuild.BuildResult{}, cleanup(err)
		}
	}
	if err := p.verifySeedImageSet(ctx, builder, plan.Images); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.guestExec(ctx, builder, "systemctl", "stop", "hacocoon-docker.service", "hacocoon-docker.socket", "containerd.service"); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("quiesce Seed runtime services: %w", err))
	}
	if err := p.expectGuestUnitState(ctx, builder, "containerd.service", "inactive"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.expectGuestUnitState(ctx, builder, "hacocoon-docker.service", "inactive"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := p.expectGuestUnitState(ctx, builder, "hacocoon-docker.socket", "inactive"); err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if _, err := p.runner.Run(ctx, "incus", "stop", builder, "--project", p.project); err != nil {
		return seedbuild.BuildResult{}, cleanup(fmt.Errorf("stop offline Seed builder: %w", err))
	}
	alias := publishedSeedAlias("seed", plan.Parent.Name)
	published, err := p.publishStoppedBuilder(ctx, builder, alias)
	if err != nil {
		return seedbuild.BuildResult{}, cleanup(err)
	}
	if err := cleanup(nil); err != nil {
		return seedbuild.BuildResult{}, errors.Join(err, core.ErrRecoveryRequired)
	}
	cleanupNeeded = false
	return published, nil
}

type toolingProvisionFiles struct {
	policyRC    string
	socketUnit  string
	serviceUnit string
}

func writeToolingProvisionFiles(dir string) (toolingProvisionFiles, error) {
	files := toolingProvisionFiles{
		policyRC:    filepath.Join(dir, "policy-rc.d"),
		socketUnit:  filepath.Join(dir, "hacocoon-docker.socket"),
		serviceUnit: filepath.Join(dir, "hacocoon-docker.service"),
	}
	if err := os.WriteFile(files.policyRC, []byte("#!/bin/sh\nexit 101\n"), 0o755); err != nil {
		return toolingProvisionFiles{}, fmt.Errorf("write package service-start policy: %w", err)
	}
	if err := os.WriteFile(files.socketUnit, []byte(hacocoonDockerSocketUnit), 0o644); err != nil {
		return toolingProvisionFiles{}, fmt.Errorf("write Docker compatibility socket unit: %w", err)
	}
	if err := os.WriteFile(files.serviceUnit, []byte(hacocoonDockerServiceUnit), 0o644); err != nil {
		return toolingProvisionFiles{}, fmt.Errorf("write Docker compatibility service unit: %w", err)
	}
	return files, nil
}

func seedNerdctlBinary() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("HACO_NERDCTL_BINARY")); configured != "" {
		info, err := os.Stat(configured)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("HACO_NERDCTL_BINARY does not name a readable binary: %w", core.ErrRuntimeUnavailable)
		}
		return configured, nil
	}
	path, err := exec.LookPath("nerdctl")
	if err != nil {
		return "", fmt.Errorf("host nerdctl is required to build Hacocoon tooling Bases; set HACO_NERDCTL_BINARY to an explicit binary: %w", core.ErrRuntimeUnavailable)
	}
	return path, nil
}

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (p *SandboxProvider) configureNestedOCIInstance(ctx context.Context, builder string) error {
	for _, key := range sortedStringKeys(nestedOCIConfig) {
		if _, err := p.runner.Run(ctx, "incus", "config", "set", builder, key+"="+nestedOCIConfig[key], "--project", p.project); err != nil {
			return fmt.Errorf("configure nested OCI builder %s: %w", key, err)
		}
	}
	return nil
}

func (p *SandboxProvider) exportSeedImages(ctx context.Context, images []seedbuild.ImageIdentity) (string, func(), error) {
	if len(images) == 0 {
		return "", func() {}, nil
	}
	dir, err := os.MkdirTemp("", "haco-seed-export-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create Seed export directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if err := os.Chmod(dir, 0o700); err != nil {
		cleanup()
		return "", func() {}, err
	}
	refs := make([]string, 0, len(images))
	for _, image := range images {
		ref := image.String()
		if _, err := p.runner.Run(ctx, "nerdctl", "--namespace", seedHostNamespace, "pull", ref); err != nil {
			cleanup()
			return "", func() {}, fmt.Errorf("acquire Seed image %s on trusted Host: %w", ref, err)
		}
		refs = append(refs, ref)
	}
	archive := filepath.Join(dir, "images.tar")
	args := []string{"--namespace", seedHostNamespace, "save", "-o", archive}
	args = append(args, refs...)
	if _, err := p.runner.Run(ctx, "nerdctl", args...); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("export trusted Host Seed images: %w", err)
	}
	info, err := os.Stat(archive)
	if err != nil || info.Size() == 0 {
		cleanup()
		return "", func() {}, fmt.Errorf("Seed OCI export archive was not created: %w", core.ErrIncompatibleState)
	}
	return archive, cleanup, nil
}

func (p *SandboxProvider) pushBuilderFile(ctx context.Context, builder, source, destination string, mode os.FileMode) error {
	target := builder + destination
	if _, err := p.runner.Run(ctx, "incus", "file", "push", source, target, "--project", p.project); err != nil {
		return fmt.Errorf("push %s into builder: %w", filepath.Base(source), err)
	}
	if err := p.guestExec(ctx, builder, "chmod", fmt.Sprintf("%#o", mode.Perm()), destination); err != nil {
		return fmt.Errorf("set builder file mode for %s: %w", destination, err)
	}
	return nil
}

func (p *SandboxProvider) guestExec(ctx context.Context, builder string, argv ...string) error {
	if len(argv) == 0 {
		return core.ErrInvalidArgument
	}
	args := append([]string{"exec", builder, "--project", p.project, "--"}, argv...)
	result, err := p.runner.Run(ctx, "incus", args...)
	if err == nil && result.ExitCode == 0 {
		return nil
	}
	reason := strings.TrimSpace(result.Stderr)
	if reason == "" && err != nil {
		reason = err.Error()
	}
	if reason == "" {
		reason = fmt.Sprintf("exit code %d", result.ExitCode)
	}
	return fmt.Errorf("guest command %q failed: %s: %w", argv[0], reason, core.ErrRuntimeUnavailable)
}

func (p *SandboxProvider) expectGuestUnitState(ctx context.Context, builder, unit, want string) error {
	args := []string{"exec", builder, "--project", p.project, "--", "systemctl", "show", "-p", "ActiveState", "--value", unit}
	result, err := p.runner.Run(ctx, "incus", args...)
	if err != nil {
		return fmt.Errorf("inspect guest unit %s: %w", unit, err)
	}
	if got := strings.TrimSpace(result.Stdout); got != want {
		return fmt.Errorf("guest unit %s state=%q want=%q: %w", unit, got, want, core.ErrIncompatibleState)
	}
	return nil
}

func (p *SandboxProvider) expectGuestUnitFileState(ctx context.Context, builder, unit, want string) error {
	args := []string{"exec", builder, "--project", p.project, "--", "systemctl", "show", "-p", "UnitFileState", "--value", unit}
	result, err := p.runner.Run(ctx, "incus", args...)
	if err != nil {
		return fmt.Errorf("inspect guest unit-file state %s: %w", unit, err)
	}
	if got := strings.TrimSpace(result.Stdout); got != want {
		return fmt.Errorf("guest unit %s unit-file-state=%q want=%q: %w", unit, got, want, core.ErrIncompatibleState)
	}
	return nil
}

func (p *SandboxProvider) verifySeedImageSet(ctx context.Context, builder string, images []seedbuild.ImageIdentity) error {
	if len(images) == 0 {
		return nil
	}
	args := []string{"exec", builder, "--project", p.project, "--", "nerdctl", "images", "--format", "{{.Repository}}\t{{.Tag}}\t{{.Digest}}"}
	result, err := p.runner.Run(ctx, "incus", args...)
	if err != nil || result.ExitCode != 0 {
		return fmt.Errorf("list imported Seed images: %w", commandResultError(result, err))
	}
	available := map[string]struct{}{}
	for _, raw := range strings.Split(result.Stdout, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			return fmt.Errorf("unexpected nerdctl image row in Seed builder: %w", core.ErrIncompatibleState)
		}
		repository := strings.TrimSpace(parts[0])
		tag := strings.TrimSpace(parts[1])
		digest := strings.ToLower(strings.TrimSpace(parts[2]))
		if repository == "<none>" || digest == "" || digest == "<none>" {
			continue
		}
		reference := repository
		if tag != "" && tag != "<none>" {
			reference += ":" + tag
		}
		available[reference+"@"+digest] = struct{}{}
	}
	for _, image := range images {
		if _, ok := available[image.String()]; !ok {
			return fmt.Errorf("Seed image %s missing after offline import: %w", image.String(), core.ErrIncompatibleState)
		}
	}
	return nil
}

func commandResultError(result host.Result, err error) error {
	reason := strings.TrimSpace(result.Stderr)
	if reason == "" && err != nil {
		reason = err.Error()
	}
	if reason == "" {
		reason = fmt.Sprintf("exit code %d", result.ExitCode)
	}
	return fmt.Errorf("%s: %w", reason, core.ErrRuntimeUnavailable)
}

func (p *SandboxProvider) verifyBuilderHasNoNIC(ctx context.Context, builder string) error {
	result, err := p.runner.Run(ctx, "incus", "config", "show", builder, "--project", p.project, "--expanded", "--format", "json")
	if err != nil {
		return fmt.Errorf("inspect offline Seed builder devices: %w", err)
	}
	var config struct {
		Devices map[string]map[string]string `json:"devices"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &config); err != nil {
		return fmt.Errorf("decode offline Seed builder devices: %w", core.ErrIncompatibleState)
	}
	for name, device := range config.Devices {
		if device["type"] == "nic" {
			return fmt.Errorf("offline Seed builder unexpectedly has NIC %q: %w", name, core.ErrIncompatibleState)
		}
	}
	return nil
}

func (p *SandboxProvider) publishStoppedBuilder(ctx context.Context, builder, alias string) (seedbuild.BuildResult, error) {
	if _, err := p.runner.Run(ctx, "incus", "publish", builder, "--project", p.project, "--alias", alias); err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("publish immutable Incus image %q: %w", alias, err)
	}
	result, err := p.runner.Run(ctx, "incus", "image", "info", "local:"+alias, "--project", p.project, "--format", "json")
	if err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("inspect published Incus image %q: %w", alias, err)
	}
	var info struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &info); err != nil {
		return seedbuild.BuildResult{}, fmt.Errorf("decode published Incus image %q: %w", alias, core.ErrIncompatibleState)
	}
	fingerprint := strings.ToLower(strings.TrimSpace(info.Fingerprint))
	if !baseFingerprintPattern.MatchString(fingerprint) {
		return seedbuild.BuildResult{}, fmt.Errorf("published Incus image %q returned invalid fingerprint: %w", alias, core.ErrIncompatibleState)
	}
	return seedbuild.BuildResult{Revision: core.BaseRevision("sha256:" + fingerprint), Alias: alias}, nil
}

func (p *SandboxProvider) verifyLocalImage(ctx context.Context, fingerprint string) error {
	result, err := p.runner.Run(ctx, "incus", "image", "info", "local:"+fingerprint, "--project", p.project, "--format", "json")
	if err != nil {
		return err
	}
	var info struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.Unmarshal([]byte(result.Stdout), &info); err != nil {
		return core.ErrIncompatibleState
	}
	if strings.ToLower(strings.TrimSpace(info.Fingerprint)) != fingerprint {
		return core.ErrIncompatibleState
	}
	return nil
}

func (p *SandboxProvider) seedBuilderCleanup(builder string) func(error) error {
	return func(cause error) error {
		ctx, cancel := context.WithTimeout(context.Background(), p.cleanupTimeout)
		defer cancel()
		_, err := p.runner.Run(ctx, "incus", "delete", builder, "--project", p.project, "--force")
		if err == nil {
			return cause
		}
		if cause == nil {
			return fmt.Errorf("cleanup Seed builder %s: %w", builder, err)
		}
		return errors.Join(cause, fmt.Errorf("cleanup Seed builder %s: %w", builder, err), core.ErrRecoveryRequired)
	}
}

func newSeedBuilderName(kind string) (string, error) {
	var random [6]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate Seed builder identity: %w", err)
	}
	return "haco-" + kind + "-build-" + hex.EncodeToString(random[:]), nil
}

func publishedSeedAlias(kind string, base core.BaseName) string {
	slug := strings.NewReplacer("/", "-", ".", "-", "_", "-").Replace(string(base))
	return fmt.Sprintf("hacocoon-%s-%s-%d", kind, slug, time.Now().UTC().UnixNano())
}

func seedFingerprint(revision core.BaseRevision) (string, error) {
	value := string(revision)
	if !strings.HasPrefix(value, "sha256:") {
		return "", core.ErrInvalidArgument
	}
	fingerprint := strings.TrimPrefix(value, "sha256:")
	if !baseFingerprintPattern.MatchString(fingerprint) {
		return "", core.ErrInvalidArgument
	}
	return fingerprint, nil
}

var _ seedbuild.Backend = (*SandboxProvider)(nil)
var _ host.Runner = host.ExecRunner{}
