//go:build linux

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	selfMountNamespace = "/proc/self/ns/mnt"
	initMountNamespace = "/proc/1/ns/mnt"
	initCommPath       = "/proc/1/comm"
	nsenterBinary      = "/usr/bin/nsenter"
	systemctlBinary    = "/usr/bin/systemctl"
	incusServiceName   = "incus.service"
)

type hostEnsureNamespaceDeps struct {
	readlink     func(string) (string, error)
	readFile     func(string) ([]byte, error)
	geteuid      func() int
	executable   func() (string, error)
	evalSymlinks func(string) (string, error)
	stat         func(string) (os.FileInfo, error)
	incusMainPID func(context.Context) (int, error)
	run          func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error
}

var defaultHostEnsureNamespaceDeps = hostEnsureNamespaceDeps{
	readlink:     os.Readlink,
	readFile:     os.ReadFile,
	geteuid:      os.Geteuid,
	executable:   os.Executable,
	evalSymlinks: filepath.EvalSymlinks,
	stat:         os.Stat,
	incusMainPID: ensureIncusMainPID,
	run: func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	},
}

func ensureIncusMainPID(ctx context.Context) (int, error) {
	readPID := func() (int, error) {
		output, err := exec.CommandContext(ctx, systemctlBinary, "show", "--property", "MainPID", "--value", incusServiceName).Output()
		if err != nil {
			return 0, err
		}
		raw := strings.TrimSpace(string(output))
		if raw == "" || raw == "0" {
			return 0, nil
		}
		pid, err := strconv.Atoi(raw)
		if err != nil || pid <= 1 {
			return 0, fmt.Errorf("invalid %s MainPID %q", incusServiceName, raw)
		}
		return pid, nil
	}

	pid, err := readPID()
	if err != nil || pid > 1 {
		return pid, err
	}
	// WSL can boot with only incus.socket active. Starting incus.service here is
	// deliberate: host ensure is about to require the daemon anyway, and we must
	// know its final mount namespace before creating the managed Btrfs mount.
	if err := exec.CommandContext(ctx, systemctlBinary, "start", incusServiceName).Run(); err != nil {
		return 0, err
	}
	pid, err = readPID()
	if err != nil {
		return 0, err
	}
	if pid <= 1 {
		return 0, fmt.Errorf("%s started without a usable MainPID", incusServiceName)
	}
	return pid, nil
}

// host ensure is the Physical Host bootstrap/recovery path. On WSL, processes
// launched by wsl.exe can inhabit a mount namespace that is different from both
// PID 1/systemd and an already-running incusd. A Btrfs mount created only in the
// session or PID 1 namespace can therefore remain invisible to incusd even
// though findmnt and the storage helper report success. For a root command that
// is actually outside PID 1's namespace, ensure Incus is running and re-enter
// its validated daemon mount namespace before composition.Local() lazily creates
// or reconciles managed storage.
func init() {
	handled, err := maybeReexecHostEnsureInInitMountNamespace(
		context.Background(),
		os.Args[1:],
		os.Stdin,
		os.Stdout,
		os.Stderr,
		defaultHostEnsureNamespaceDeps,
	)
	if !handled {
		return
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
			os.Exit(exitErr.ExitCode())
		}
		fail(err)
	}
	os.Exit(0)
}

func maybeReexecHostEnsureInInitMountNamespace(
	ctx context.Context,
	args []string,
	stdin io.Reader,
	stdout, stderr io.Writer,
	deps hostEnsureNamespaceDeps,
) (bool, error) {
	if len(args) != 2 || args[0] != "host" || args[1] != "ensure" {
		return false, nil
	}
	if deps.geteuid == nil {
		return true, fmt.Errorf("invalid host namespace bootstrap dependency")
	}

	// Normal users already have a supported helper-mediated host-ensure path.
	// They also may not be permitted to inspect system mount namespace handles on
	// hardened hosts. Namespace rebinding is only required for the root
	// installer/bootstrap invocation that can enter those namespaces.
	if deps.geteuid() != 0 {
		return false, nil
	}
	if deps.readlink == nil || deps.readFile == nil || deps.executable == nil || deps.evalSymlinks == nil || deps.stat == nil || deps.incusMainPID == nil || deps.run == nil || stdin == nil || stdout == nil || stderr == nil {
		return true, fmt.Errorf("invalid host namespace bootstrap dependency")
	}

	selfNS, err := deps.readlink(selfMountNamespace)
	if err != nil {
		return true, fmt.Errorf("inspect current mount namespace: %w", err)
	}
	initNS, err := deps.readlink(initMountNamespace)
	if err != nil {
		return true, fmt.Errorf("inspect PID 1 mount namespace: %w", err)
	}
	// Preserve the ordinary Linux path exactly. The Incus-daemon namespace
	// workaround is only needed for WSL-style root commands that already differ
	// from the systemd/PID 1 mount namespace.
	if selfNS == initNS {
		return false, nil
	}

	comm, err := deps.readFile(initCommPath)
	if err != nil {
		return true, fmt.Errorf("inspect PID 1 before Physical Host namespace entry: %w", err)
	}
	if strings.TrimSpace(string(comm)) != "systemd" {
		return true, fmt.Errorf("refusing Physical Host mount namespace entry because PID 1 is %q, not systemd", strings.TrimSpace(string(comm)))
	}

	pid, err := deps.incusMainPID(ctx)
	if err != nil {
		return true, fmt.Errorf("resolve running %s for WSL mount reconciliation: %w", incusServiceName, err)
	}
	if pid <= 1 {
		return true, fmt.Errorf("resolve running %s for WSL mount reconciliation: invalid MainPID %d", incusServiceName, pid)
	}
	commPath := fmt.Sprintf("/proc/%d/comm", pid)
	incusComm, err := deps.readFile(commPath)
	if err != nil {
		return true, fmt.Errorf("inspect %s MainPID %d before mount namespace entry: %w", incusServiceName, pid, err)
	}
	if strings.TrimSpace(string(incusComm)) != "incusd" {
		return true, fmt.Errorf("refusing %s mount namespace entry because MainPID %d is %q, not incusd", incusServiceName, pid, strings.TrimSpace(string(incusComm)))
	}
	targetNamespace := fmt.Sprintf("/proc/%d/ns/mnt", pid)

	targetNS, err := deps.readlink(targetNamespace)
	if err != nil {
		return true, fmt.Errorf("inspect target mount namespace %s: %w", targetNamespace, err)
	}
	if selfNS == targetNS {
		return false, nil
	}

	executable, err := deps.executable()
	if err != nil {
		return true, fmt.Errorf("resolve haco executable for Physical Host namespace entry: %w", err)
	}
	executable, err = deps.evalSymlinks(executable)
	if err != nil {
		return true, fmt.Errorf("resolve haco executable symlinks for Physical Host namespace entry: %w", err)
	}
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable {
		return true, fmt.Errorf("resolved haco executable is not an absolute clean path: %q", executable)
	}
	if _, err := deps.stat(nsenterBinary); err != nil {
		return true, fmt.Errorf("Physical Host namespace entry requires %s: %w", nsenterBinary, err)
	}

	nsenterArgs := []string{
		"--mount=" + targetNamespace,
		"--",
		executable,
		"host",
		"ensure",
	}
	if err := deps.run(ctx, nsenterBinary, nsenterArgs, stdin, stdout, stderr); err != nil {
		return true, err
	}
	return true, nil
}
