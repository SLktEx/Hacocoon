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
	"strings"
)

const (
	selfMountNamespace = "/proc/self/ns/mnt"
	initMountNamespace = "/proc/1/ns/mnt"
	initCommPath       = "/proc/1/comm"
	nsenterBinary      = "/usr/bin/nsenter"
)

type hostEnsureNamespaceDeps struct {
	readlink   func(string) (string, error)
	readFile   func(string) ([]byte, error)
	geteuid    func() int
	executable func() (string, error)
	run        func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error
}

var defaultHostEnsureNamespaceDeps = hostEnsureNamespaceDeps{
	readlink:   os.Readlink,
	readFile:   os.ReadFile,
	geteuid:    os.Geteuid,
	executable: os.Executable,
	run: func(ctx context.Context, name string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdin = stdin
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		return cmd.Run()
	},
}

// host ensure is the Physical Host bootstrap/recovery path. On WSL, processes
// launched by wsl.exe can inhabit a mount namespace that is different from the
// PID 1/systemd namespace that owns incusd. A Btrfs mount created only in the
// session namespace is therefore invisible to incusd even though mount(2) and
// the storage helper both report success. Re-enter PID 1's mount namespace
// before composition.Local() can lazily create or reconcile managed storage.
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
	if deps.readlink == nil || deps.readFile == nil || deps.geteuid == nil || deps.executable == nil || deps.run == nil || stdin == nil || stdout == nil || stderr == nil {
		return true, coreInvalidNamespaceDependency()
	}

	selfNS, err := deps.readlink(selfMountNamespace)
	if err != nil {
		return true, fmt.Errorf("inspect current mount namespace: %w", err)
	}
	initNS, err := deps.readlink(initMountNamespace)
	if err != nil {
		return true, fmt.Errorf("inspect PID 1 mount namespace: %w", err)
	}
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
	if deps.geteuid() != 0 {
		return true, fmt.Errorf("haco host ensure must run as root to enter the Physical Host mount namespace")
	}

	executable, err := deps.executable()
	if err != nil {
		return true, fmt.Errorf("resolve haco executable for Physical Host namespace entry: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return true, fmt.Errorf("resolve haco executable symlinks for Physical Host namespace entry: %w", err)
	}
	if !filepath.IsAbs(executable) || filepath.Clean(executable) != executable {
		return true, fmt.Errorf("resolved haco executable is not an absolute clean path: %q", executable)
	}
	if _, err := os.Stat(nsenterBinary); err != nil {
		return true, fmt.Errorf("Physical Host namespace entry requires %s: %w", nsenterBinary, err)
	}

	nsenterArgs := []string{
		"--mount=" + initMountNamespace,
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

func coreInvalidNamespaceDependency() error {
	return fmt.Errorf("invalid host namespace bootstrap dependency")
}
