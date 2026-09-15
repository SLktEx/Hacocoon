"""Real product stdin/TTY acceptance, restricted to the disposable Incus CI host."""

import errno
import fcntl
import os
import pty
import select
import signal
import struct
import subprocess
import termios
import time


def _native_names():
    result = subprocess.run(
        ["incus", "list", "--project", "hacocoon", "--format", "csv", "-c", "n"],
        capture_output=True, text=True, timeout=15,
    )
    if result.returncode:
        raise RuntimeError("could not inspect streamed-run provider ownership")
    return set(result.stdout.splitlines())


def verify_streaming(product, workspace, rows):
    if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("HACO_CI_RUNNER_ENVIRONMENT") != "github-hosted":
        raise RuntimeError("stream acceptance requires the disposable GHA host")
    baseline, native = rows(), _native_names()
    data = bytes(range(256)) * 8192
    result = subprocess.run(
        [product, "run", "-i", "--workspace", workspace, "--", "sh", "-c",
         'test "$PWD" = /workspace || exit 99; cat; printf stream-stderr >&2; exit 17'],
        input=data, capture_output=True, timeout=300,
    )
    if result.returncode != 17 or result.stdout != data or result.stderr != b"stream-stderr":
        raise RuntimeError("piped product input/output/exit mismatch")
    if rows() != baseline or _native_names() != native:
        raise RuntimeError("streamed run did not preserve baseline ownership")

    master, slave = pty.openpty()
    before = termios.tcgetattr(slave)
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
    command = (
        'test -t 0; test "$PWD" = /workspace; printf "TTY-READY\\n"; stty size; '
        'IFS= read -r line; printf "INPUT:%s\\n" "$line"; '
        'while [ "$(stty size)" != "43 132" ]; do sleep 0.1; done; '
        'printf "RESIZED:"; stty size; exit 17'
    )
    process = subprocess.Popen(
        [product, "run", "-it", "--workspace", workspace, "--", "sh", "-ec", command],
        stdin=slave, stdout=slave, stderr=slave, start_new_session=True,
    )
    transcript = bytearray()

    def pump(until, predicate):
        while not predicate():
            if time.monotonic() >= until:
                raise RuntimeError("product terminal observation timed out")
            ready, _, _ = select.select([master], [], [], 0.2)
            if ready:
                try:
                    part = os.read(master, 4096)
                except OSError as exc:
                    if exc.errno != errno.EIO:
                        raise
                    part = b""
                if part:
                    transcript.extend(part)
                    if len(transcript) > 65536:
                        raise RuntimeError("terminal transcript exceeded its bound")
            if process.poll() is not None and not predicate() and (not ready or not part):
                raise RuntimeError("product terminal exited before its expected observation")

    try:
        pump(time.monotonic() + 300, lambda: b"TTY-READY" in transcript and b"24 80" in transcript)
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 43, 132, 0, 0))
        process.send_signal(signal.SIGWINCH)
        os.write(master, b"abc\x7fD\n")
        pump(time.monotonic() + 60, lambda: b"INPUT:abD" in transcript and b"RESIZED:43 132" in transcript)
        process.wait(timeout=45)
        if process.returncode != 17:
            raise RuntimeError("product terminal did not preserve exit 17")
        if termios.tcgetattr(slave) != before:
            raise RuntimeError("product terminal settings were not restored")
    finally:
        if process.poll() is None:
            process.send_signal(signal.SIGTERM)
            try:
                process.wait(timeout=45)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=10)
        os.close(master)
        os.close(slave)
    if rows() != baseline or _native_names() != native:
        raise RuntimeError("terminal run did not preserve baseline ownership")
    print("TEMPORARY STREAM / BINARY PIPE / REAL PTY EDIT-RESIZE / EXIT 17 / RESTORE / CLEANUP: PASS")
