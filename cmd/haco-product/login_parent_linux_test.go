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
	script := `import ctypes, os, pty, select, subprocess, sys, time
from pathlib import Path
libc = ctypes.CDLL(None)
env = dict(os.environ, HOME=sys.argv[2], HACO_TEST_LOGIN_BOOTSTRAP="1", HACO_CONTROL_SOCKET="/nonexistent-haco-login-regression.sock")
for name, expected in ((b"login", 42), (b"init", 43), (b"login-helper", 43)):
    assert libc.prctl(15, name, 0, 0, 0) == 0
    result = subprocess.run([sys.argv[1], "-test.run=^TestLoginBootstrapHelperProcess$"], env=dict(env, HACO_TEST_LOGIN_PARENT_ONLY="1"), timeout=3)
    assert result.returncode == expected
assert libc.prctl(15, b"login", 0, 0, 0) == 0
# The fixture owns its login profile. Wait for the actual Bash input prompt:
# writing into the PTY before shell/readline startup can lose type-ahead input.
# Ubuntu's global profile otherwise runs update-motd for each fresh HOME,
# including unrelated system inventory and update checks before this profile.
# Use the normal per-user opt-out; keep the real login shell and its deadline.
Path(sys.argv[2], ".hushlogin").touch()
Path(sys.argv[2], ".bash_profile").write_text("PS1='__HACO_LOGIN_READY__ '\n")
master, slave = pty.openpty()
process = subprocess.Popen([sys.argv[1], "-test.run=^TestLoginBootstrapHelperProcess$"], stdin=slave, stdout=slave, stderr=slave, env=env)
try:
    deadline = time.monotonic() + 5
    output = bytearray()
    while b"__HACO_LOGIN_READY__ " not in output:
        remaining = deadline - time.monotonic()
        assert remaining > 0, "bootstrap never reached the Bash input prompt"
        ready, _, _ = select.select([master], [], [], remaining)
        assert ready, "bootstrap never reached the Bash input prompt"
        output.extend(os.read(master, 4096))
        assert len(output) <= 65536, "unexpected login output size"
    os.write(master, b"exit 37\n")
    assert process.wait(timeout=max(.001, deadline-time.monotonic())) == 37, "bootstrap must enter bash without contacting the controller"
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
