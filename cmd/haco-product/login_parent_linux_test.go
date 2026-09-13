//go:build linux

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestLoginBootstrapHelperProcess(t *testing.T) {
	if os.Getenv("HACO_TEST_LOGIN_BOOTSTRAP") != "1" {
		return
	}
	if os.Getenv("HACO_TEST_LOGIN_PARENT_ONLY") == "1" {
		bootstrap, err := loginBootstrapParent()
		if err != nil {
			os.Exit(1)
		}
		if bootstrap {
			os.Exit(42)
		}
		os.Exit(43)
	}
	if err := runLoginShim(nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestLoginBootstrapPTYDoesNotStartHostSetup(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	// Emulate WSL's distinct login parent on an actual Linux PTY. No PAM,
	// account, service, real controller or Incus mutation is involved.
	script := `import ctypes, os, pty, subprocess, sys
libc = ctypes.CDLL(None)
env = dict(os.environ, HOME=sys.argv[2], HACO_TEST_LOGIN_BOOTSTRAP="1", HACO_CONTROL_SOCKET="/nonexistent-haco-login-regression.sock")
for name, expected in ((b"login", 42), (b"init", 43), (b"login-helper", 43)):
    assert libc.prctl(15, name, 0, 0, 0) == 0
    result = subprocess.run([sys.argv[1], "-test.run=^TestLoginBootstrapHelperProcess$"], env=dict(env, HACO_TEST_LOGIN_PARENT_ONLY="1"), timeout=3)
    assert result.returncode == expected
assert libc.prctl(15, b"login", 0, 0, 0) == 0
master, slave = pty.openpty()
process = subprocess.Popen([sys.argv[1], "-test.run=^TestLoginBootstrapHelperProcess$"], stdin=slave, stdout=slave, stderr=slave, env=env)
try:
    os.write(master, b"exit 37\n")
    assert process.wait(timeout=5) == 37, "bootstrap must enter bash without contacting the controller"
finally:
    if process.poll() is None:
        process.kill()
        process.wait()
    os.close(slave)
    os.close(master)
`
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-c", script, os.Args[0], t.TempDir())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("login bootstrap: %v: %s", err, output)
	}
}
