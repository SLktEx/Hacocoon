#!/usr/bin/env python3
"""Exercise the built haco binary's Physical Host login routes on Linux.

No controller, provider, account, or installed shell is changed. This verifies
executable-name dispatch; real trusted-Host/WSL acceptance remains in the
installed journeys. Run with: python3 test/e2e/login.py /path/to/built/haco
"""

import ctypes
import errno
import os
from pathlib import Path
import pty
import select
import signal
import subprocess
import sys
import tempfile
import termios


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def run_entry(executable, argv, env, *, script=None, input_tty=False, output_tty=False):
    master = slave = None
    process = None
    try:
        if input_tty or output_tty:
            master, slave = pty.openpty()
            attributes = termios.tcgetattr(slave)
            attributes[1] &= ~termios.OPOST
            attributes[3] &= ~termios.ECHO
            termios.tcsetattr(slave, termios.TCSANOW, attributes)
        process = subprocess.Popen(
            argv, executable=str(executable), env=env, cwd=env["HOME"],
            stdin=slave if input_tty else subprocess.PIPE,
            stdout=slave if output_tty else subprocess.PIPE,
            stderr=subprocess.PIPE, start_new_session=True,
        )
        if slave is not None:
            os.close(slave)
            slave = None
        if input_tty and script is not None:
            require(os.write(master, script) == len(script), "incomplete terminal input")
        stdout, stderr = process.communicate(
            input=None if input_tty else script, timeout=5,
        )
        if output_tty:
            stdout = b""
            while select.select([master], [], [], 0)[0]:
                try:
                    chunk = os.read(master, 4096)
                except OSError as error:
                    if error.errno == errno.EIO:
                        break
                    raise
                if not chunk:
                    break
                stdout += chunk
                require(len(stdout) <= 65536, "terminal output exceeded fixture limit")
        return process.returncode, stdout, stderr
    finally:
        if process is not None:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGKILL)
            process.wait()
        for descriptor in (slave, master):
            if descriptor is not None:
                os.close(descriptor)


def check_result(name, result, *, code=37, stdout=b"physical-host\n", stderr=b""):
    # Report only categories, never arbitrary subprocess output or environment.
    require(result[0] == code, f"{name}: exit status {result[0]}, expected {code}")
    require(result[1] == stdout, f"{name}: stdout differed from exact command output")
    require(result[2] == stderr, f"{name}: stderr differed from exact command output")
    print(f"Login executable routing / {name}: PASS", flush=True)


def main(binary):
    binary = Path(binary).resolve(strict=True)
    require(binary.is_file() and os.access(binary, os.X_OK), "haco must be executable")
    with tempfile.TemporaryDirectory(prefix="haco-login-entry-") as temporary:
        root = Path(temporary)
        home = root / "home"
        home.mkdir()
        # Login Bash may otherwise run Ubuntu's unrelated MOTD inventory.
        (home / ".hushlogin").touch()
        alias = root / "entry with spaces" / "hacocoon-login"
        alias.parent.mkdir()
        alias.symlink_to(binary)
        # A clean environment prevents inherited shell hooks, credentials or
        # real controller configuration from participating in this fixture.
        env = {
            "PATH": "/usr/bin:/bin", "HOME": str(home), "LC_ALL": "C",
            "TERM": "dumb", "NO_COLOR": "1", "SHELL": str(alias),
            "HACO_ROOT": str(root / "must-not-create"),
            "HACO_CONTROL_SOCKET": str(root / "missing-controller.sock"),
        }
        script = b"printf 'physical-host\\n'; exit 37\n"
        command = "printf '%s\\n' \"$1\"; printf 'command-error\\n' >&2; exit 37"
        # Arguments must pass through verbatim, even when stdin/stdout are TTYs.
        value = "literal spaces; $(not-a-command) 'quotes' --option"
        for language in ("en", "ja"):
            env["HACO_UI_LANGUAGE"] = language
            for terminal in (False, True):
                check_result(
                    f"explicit-command-{language}-tty-{terminal}",
                    run_entry(alias, [str(alias), "-c", command, "fixture", value], env,
                              input_tty=terminal, output_tty=terminal),
                    stdout=(value + "\n").encode(), stderr=b"command-error\n",
                )
        check_result(
            "login-prefixed-argv0",
            run_entry(alias, ["-hacocoon-login", "-c", script.decode()], env),
        )
        for input_tty, output_tty in ((False, False), (True, False), (False, True)):
            check_result(
                f"noninteractive-input-tty-{input_tty}-output-tty-{output_tty}",
                run_entry(alias, [str(alias)], env, script=script,
                          input_tty=input_tty, output_tty=output_tty),
            )
        check_result("noninteractive-EOF", run_entry(alias, [str(alias)], env),
                     code=0, stdout=b"")

        # WSL's PAM bootstrap also has a PTY. Emulate only its immediate parent
        # command identity; this neither emulates PAM nor grants Host authority.
        libc = ctypes.CDLL(None, use_errno=True)
        parent_name = Path("/proc/self/comm").read_bytes().rstrip(b"\n")
        require(libc.prctl(15, b"login", 0, 0, 0) == 0, "cannot name bootstrap parent")
        try:
            check_result(
                "login-parent-PTY",
                run_entry(alias, [str(alias)], env, input_tty=True, output_tty=True,
                          script=b"shopt -q login_shell || exit 9; printf 'physical-host\\n'; exit 37\n"),
            )
        finally:
            require(libc.prctl(15, parent_name, 0, 0, 0) == 0, "cannot restore parent name")

        # A near-match name must retain ordinary product command routing.
        lookalike = alias.with_name("hacocoon-login-helper")
        lookalike.symlink_to(binary)
        version = run_entry(binary, [str(binary), "--version"], env)
        require(version[0] == 0 and version[1].startswith(b"haco ") and not version[2],
                "ordinary haco version entry failed")
        check_result("alias-near-match", run_entry(lookalike, [str(lookalike), "--version"], env),
                     code=0, stdout=version[1])
        require(not Path(env["HACO_ROOT"]).exists(), "login routing created local product state")
        require(not Path(env["HACO_CONTROL_SOCKET"]).exists(), "login routing created a controller socket")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: login.py /path/to/built/haco")
    main(sys.argv[1])
