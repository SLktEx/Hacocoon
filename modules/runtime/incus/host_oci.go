package incus

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/SLktEx/Hacocoon/internal/core"
	"github.com/SLktEx/Hacocoon/internal/host"
)

const (
	trustedHostNerdctlVersion     = "2.3.5"
	trustedHostNerdctlAMD64SHA256 = "de3206aeb7cbd5f20f5fb1f55c1e3bf2db1be567812a8a3f5e65eba2488347ee"
	trustedHostNerdctlPath        = "/usr/local/bin/nerdctl"
	trustedHostTransferDir        = "/var/lib/hacocoon/oci-transfer"
)

// TrustedHostOCIRunner keeps Incus authority on the Physical Host while routing
// Host OCI tooling into the persistent trusted haco-host instance. It is
// intentionally narrow: only Incus platform operations and nerdctl OCI-store
// operations are accepted.
type TrustedHostOCIRunner struct {
	runtime  *Runtime
	physical host.Runner

	mu    sync.Mutex
	ready bool
}

func NewTrustedHostOCIRunner(runtime *Runtime) *TrustedHostOCIRunner {
	if runtime == nil {
		return nil
	}
	return &TrustedHostOCIRunner{runtime: runtime, physical: runtime.runner}
}

func (r *TrustedHostOCIRunner) Run(ctx context.Context, name string, args ...string) (host.Result, error) {
	if r == nil || r.runtime == nil || r.physical == nil {
		return host.Result{ExitCode: -1}, core.ErrRuntimeUnavailable
	}
	switch name {
	case "incus":
		return r.physical.Run(ctx, name, args...)
	case "nerdctl":
		if err := r.ensureReady(ctx); err != nil {
			return host.Result{ExitCode: -1}, err
		}
		if output, ok, err := trustedHostSaveOutput(args); err != nil {
			return host.Result{ExitCode: -1}, err
		} else if ok {
			return r.runSave(ctx, args, output)
		}
		if input, ok, err := trustedHostLoadInput(args); err != nil {
			return host.Result{ExitCode: -1}, err
		} else if ok {
			return r.runLoad(ctx, args, input)
		}
		return r.exec(ctx, name, args...)
	default:
		return host.Result{ExitCode: -1}, fmt.Errorf("trusted haco-host runner refuses Physical Host command %q: %w", name, core.ErrUnsupported)
	}
}

// MaterializeNerdctlBinary makes the verified haco-host nerdctl client
// available as a short-lived Physical-Host staging file. The Physical Host does
// not execute the binary; the copy exists only so the trusted tooling Base
// builder can receive the same pinned client.
func (r *TrustedHostOCIRunner) MaterializeNerdctlBinary(ctx context.Context) (string, func(), error) {
	if r == nil || r.runtime == nil || r.physical == nil {
		return "", func() {}, core.ErrRuntimeUnavailable
	}
	if err := r.ensureReady(ctx); err != nil {
		return "", func() {}, err
	}
	dir, err := os.MkdirTemp("", "haco-host-nerdctl-*")
	if err != nil {
		return "", func() {}, fmt.Errorf("create trusted nerdctl staging directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if err := os.Chmod(dir, 0o700); err != nil {
		cleanup()
		return "", func() {}, err
	}
	path := filepath.Join(dir, "nerdctl")
	result, pullErr := r.physical.Run(ctx, "incus", "file", "pull",
		trustedHostName+trustedHostNerdctlPath, path, "--project", r.runtime.project)
	if pullErr != nil || result.ExitCode != 0 {
		cleanup()
		return "", func() {}, fmt.Errorf("stage nerdctl from trusted haco-host: %w", commandResultError(result, pullErr))
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		cleanup()
		return "", func() {}, fmt.Errorf("trusted haco-host nerdctl staging file is invalid: %w", core.ErrIncompatibleState)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}

func (r *TrustedHostOCIRunner) ensureReady(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.ready {
		return nil
	}
	if err := r.runtime.EnsureTrustedHostOCI(ctx); err != nil {
		return err
	}
	r.ready = true
	return nil
}

// EnsureTrustedHostOCI reconciles the optional Host OCI store inside haco-host.
// The Physical Host remains an Incus/storage/controller platform and does not
// need containerd or nerdctl installed.
func (r *Runtime) EnsureTrustedHostOCI(ctx context.Context) error {
	if r == nil || r.runner == nil {
		return core.ErrRuntimeUnavailable
	}
	if err := r.EnsureTrustedHost(ctx); err != nil {
		return err
	}
	if ok := r.trustedHostOCIReady(ctx); ok {
		return nil
	}

	if err := r.trustedHostExec(ctx, "apt-get", "update"); err != nil {
		return fmt.Errorf("update haco-host OCI package metadata: %w", err)
	}
	if err := r.trustedHostExec(ctx, "env", "DEBIAN_FRONTEND=noninteractive", "apt-get", "install", "-y",
		"ca-certificates", "curl", "containerd", "containernetworking-plugins"); err != nil {
		return fmt.Errorf("install haco-host OCI packages: %w", err)
	}
	if err := r.installTrustedHostNerdctl(ctx); err != nil {
		return err
	}
	if err := r.trustedHostExec(ctx, "systemctl", "enable", "--now", "containerd.service"); err != nil {
		return fmt.Errorf("enable haco-host containerd: %w", err)
	}
	if !r.trustedHostOCIReady(ctx) {
		return fmt.Errorf("haco-host OCI tooling did not converge: %w", core.ErrIncompatibleState)
	}
	return nil
}

func (r *Runtime) trustedHostOCIReady(ctx context.Context) bool {
	containerd, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "systemctl", "is-active", "containerd.service")
	if err != nil || containerd.ExitCode != 0 || strings.TrimSpace(containerd.Stdout) != "active" {
		return false
	}
	nerdctl, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", trustedHostNerdctlPath, "--version")
	if err != nil || nerdctl.ExitCode != 0 {
		return false
	}
	return strings.Contains(strings.TrimSpace(nerdctl.Stdout), "nerdctl version "+trustedHostNerdctlVersion)
}

func (r *Runtime) installTrustedHostNerdctl(ctx context.Context) error {
	arch, err := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project, "--", "uname", "-m")
	if err != nil || arch.ExitCode != 0 {
		return fmt.Errorf("inspect haco-host architecture: %w", commandResultError(arch, err))
	}
	if strings.TrimSpace(arch.Stdout) != "x86_64" {
		return fmt.Errorf("pinned haco-host nerdctl provisioning currently supports x86_64 only: %w", core.ErrUnsupported)
	}

	archive := "/tmp/nerdctl-" + trustedHostNerdctlVersion + "-linux-amd64.tar.gz"
	url := "https://github.com/containerd/nerdctl/releases/download/v" + trustedHostNerdctlVersion + "/nerdctl-" + trustedHostNerdctlVersion + "-linux-amd64.tar.gz"
	if err := r.trustedHostExec(ctx, "curl", "--fail", "--silent", "--show-error", "--location",
		"--proto", "=https", "--tlsv1.2", url, "--output", archive); err != nil {
		return fmt.Errorf("download pinned haco-host nerdctl: %w", err)
	}
	defer func() { _ = r.trustedHostExec(context.Background(), "rm", "-f", archive) }()

	hash, hashErr := r.runner.Run(ctx, "incus", "exec", trustedHostName, "--project", r.project,
		"--", "sha256sum", archive)
	if hashErr != nil || hash.ExitCode != 0 {
		return fmt.Errorf("hash downloaded haco-host nerdctl: %w", commandResultError(hash, hashErr))
	}
	fields := strings.Fields(hash.Stdout)
	if len(fields) < 1 || fields[0] != trustedHostNerdctlAMD64SHA256 {
		return fmt.Errorf("downloaded haco-host nerdctl checksum mismatch: %w", core.ErrIncompatibleState)
	}
	if err := r.trustedHostExec(ctx, "tar", "-xzf", archive, "-C", "/usr/local/bin", "nerdctl"); err != nil {
		return fmt.Errorf("install pinned haco-host nerdctl: %w", err)
	}
	if err := r.trustedHostExec(ctx, "chmod", "0755", trustedHostNerdctlPath); err != nil {
		return err
	}
	return nil
}

func (r *Runtime) trustedHostExec(ctx context.Context, argv ...string) error {
	if len(argv) == 0 {
		return core.ErrInvalidArgument
	}
	args := append([]string{"exec", trustedHostName, "--project", r.project, "--"}, argv...)
	result, err := r.runner.Run(ctx, "incus", args...)
	if err == nil && result.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("haco-host command %q failed: %w", argv[0], commandResultError(result, err))
}

func (r *TrustedHostOCIRunner) exec(ctx context.Context, name string, args ...string) (host.Result, error) {
	execArgs := []string{"exec", trustedHostName, "--project", r.runtime.project, "--", name}
	execArgs = append(execArgs, args...)
	return r.physical.Run(ctx, "incus", execArgs...)
}

func (r *TrustedHostOCIRunner) runSave(ctx context.Context, args []string, output string) (host.Result, error) {
	if err := validateTrustedTransferDestination(output); err != nil {
		return host.Result{ExitCode: -1}, err
	}
	guest, err := trustedTransferPath()
	if err != nil {
		return host.Result{ExitCode: -1}, err
	}
	if err := r.ensureTransferDir(ctx); err != nil {
		return host.Result{ExitCode: -1}, err
	}
	defer r.cleanupGuestTransfer(guest)

	rewritten := replaceFlagValue(args, "-o", guest)
	result, runErr := r.exec(ctx, "nerdctl", rewritten...)
	if runErr != nil || result.ExitCode != 0 {
		return result, runErr
	}
	pullResult, pullErr := r.physical.Run(ctx, "incus", "file", "pull",
		trustedHostName+guest, output, "--project", r.runtime.project)
	if pullErr != nil || pullResult.ExitCode != 0 {
		return pullResult, fmt.Errorf("copy OCI archive from trusted haco-host: %w", commandResultError(pullResult, pullErr))
	}
	info, statErr := os.Lstat(output)
	if statErr != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return host.Result{ExitCode: -1}, fmt.Errorf("trusted haco-host OCI export is invalid: %w", core.ErrIncompatibleState)
	}
	return result, nil
}

func (r *TrustedHostOCIRunner) runLoad(ctx context.Context, args []string, input string) (host.Result, error) {
	if err := validateTrustedTransferSource(input); err != nil {
		return host.Result{ExitCode: -1}, err
	}
	guest, err := trustedTransferPath()
	if err != nil {
		return host.Result{ExitCode: -1}, err
	}
	if err := r.ensureTransferDir(ctx); err != nil {
		return host.Result{ExitCode: -1}, err
	}
	defer r.cleanupGuestTransfer(guest)

	pushResult, pushErr := r.physical.Run(ctx, "incus", "file", "push", input,
		trustedHostName+guest, "--project", r.runtime.project, "--uid", "0", "--gid", "0", "--mode", "0600")
	if pushErr != nil || pushResult.ExitCode != 0 {
		return pushResult, fmt.Errorf("copy OCI archive into trusted haco-host: %w", commandResultError(pushResult, pushErr))
	}
	rewritten := replaceFlagValue(args, "-i", guest)
	return r.exec(ctx, "nerdctl", rewritten...)
}

func (r *TrustedHostOCIRunner) ensureTransferDir(ctx context.Context) error {
	return r.runtime.trustedHostExec(ctx, "install", "-d", "-m", "0700", trustedHostTransferDir)
}

func (r *TrustedHostOCIRunner) cleanupGuestTransfer(path string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultCleanupTimeout)
	defer cancel()
	_, _ = r.exec(ctx, "rm", "-f", path)
}

func trustedHostSaveOutput(args []string) (string, bool, error) {
	if len(args) == 0 || args[0] != "save" {
		return "", false, nil
	}
	value, ok := flagValue(args, "-o")
	if !ok {
		return "", false, fmt.Errorf("trusted haco-host nerdctl save requires -o staging path: %w", core.ErrInvalidArgument)
	}
	return value, true, nil
}

func trustedHostLoadInput(args []string) (string, bool, error) {
	start := 0
	for start < len(args) && strings.HasPrefix(args[start], "-") {
		if args[start] == "--namespace" && start+1 < len(args) {
			start += 2
			continue
		}
		break
	}
	if start >= len(args) || args[start] != "load" {
		return "", false, nil
	}
	value, ok := flagValue(args[start:], "-i")
	if !ok {
		return "", false, fmt.Errorf("trusted haco-host nerdctl load requires -i staging path: %w", core.ErrInvalidArgument)
	}
	return value, true, nil
}

func flagValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args); i++ {
		if args[i] == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func replaceFlagValue(args []string, flag, value string) []string {
	result := append([]string(nil), args...)
	for i := 0; i+1 < len(result); i++ {
		if result[i] == flag {
			result[i+1] = value
			return result
		}
	}
	return result
}

func validateTrustedTransferDestination(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("invalid Physical Host OCI export path %q: %w", path, core.ErrInvalidArgument)
	}
	info, err := os.Lstat(filepath.Dir(path))
	if err != nil || !info.IsDir() || info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("unsafe Physical Host OCI export directory %q: %w", filepath.Dir(path), core.ErrIncompatibleState)
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("Physical Host OCI export path already exists %q: %w", path, core.ErrIncompatibleState)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func validateTrustedTransferSource(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path || strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("invalid Physical Host OCI import path %q: %w", path, core.ErrInvalidArgument)
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return fmt.Errorf("invalid Physical Host OCI import file %q: %w", path, core.ErrIncompatibleState)
	}
	return nil
}

func trustedTransferPath() (string, error) {
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate trusted haco-host transfer identity: %w", err)
	}
	return trustedHostTransferDir + "/" + hex.EncodeToString(raw[:]) + ".tar", nil
}

var _ host.Runner = (*TrustedHostOCIRunner)(nil)
