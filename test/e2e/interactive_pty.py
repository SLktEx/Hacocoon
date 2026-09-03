#!/usr/bin/env python3
"""Exercise a Hacocoon interactive shell through a real local pseudo-terminal."""

from __future__ import annotations

import argparse
import fcntl
import os
import pty
import select
import signal
import struct
import subprocess
import sys
import termios
import time


class PTYSession:
    def __init__(self, argv: list[str], rows: int = 32, columns: int = 100) -> None:
        self.master, self.slave = pty.openpty()
        self.original = termios.tcgetattr(self.slave)
        fcntl.ioctl(self.slave, termios.TIOCSWINSZ, struct.pack("HHHH", rows, columns, 0, 0))
        env = os.environ.copy()
        env.setdefault("TERM", "xterm-256color")
        self.process = subprocess.Popen(
            argv,
            stdin=self.slave,
            stdout=self.slave,
            stderr=self.slave,
            env=env,
            close_fds=True,
        )
        self.buffer = bytearray()
        self.transcript = bytearray()

    def close(self) -> None:
        for fd in (self.master, self.slave):
            try:
                os.close(fd)
            except OSError:
                pass

    def send(self, data: bytes) -> None:
        os.write(self.master, data)

    def read_until(self, needle: bytes, timeout: float = 10.0) -> bytes:
        deadline = time.monotonic() + timeout
        while True:
            position = self.buffer.find(needle)
            if position >= 0:
                end = position + len(needle)
                found = bytes(self.buffer[:end])
                del self.buffer[:end]
                return found

            remaining = deadline - time.monotonic()
            if remaining <= 0:
                self.fail(f"timed out waiting for {needle!r}")

            readable, _, _ = select.select([self.master], [], [], remaining)
            if not readable:
                continue
            try:
                chunk = os.read(self.master, 4096)
            except OSError as exc:
                if self.process.poll() is not None:
                    self.fail(f"PTY closed while waiting for {needle!r}: {exc}")
                raise
            if not chunk:
                self.fail(f"PTY reached EOF while waiting for {needle!r}")
            self.buffer.extend(chunk)
            self.transcript.extend(chunk)

    def assert_raw(self) -> None:
        current = termios.tcgetattr(self.slave)
        lflag = current[3]
        if lflag & (termios.ECHO | termios.ICANON):
            self.fail("local terminal did not enter raw/no-echo mode")

    def assert_restored(self) -> None:
        current = termios.tcgetattr(self.slave)
        if current != self.original:
            self.fail("local terminal attributes were not restored")

    def fail(self, message: str) -> None:
        transcript = bytes(self.transcript + self.buffer).decode("utf-8", "replace")
        raise RuntimeError(f"{message}\n--- PTY transcript ---\n{transcript}\n--- end transcript ---")


def exercise_interactive(session: PTYSession, prompt_label: bytes) -> None:
    session.read_until(prompt_label)
    session.assert_raw()

    session.send(b'n=$(( ${n:-0}+1 )); printf "HACO_HISTORY_COUNT:%s\\n" "$n"\n')
    session.read_until(b"HACO_HISTORY_COUNT:1")
    session.read_until(prompt_label)

    # Readline/history must receive the escape sequence immediately. This is the
    # exact user action that canonical-mode client stdin used to break.
    session.send(b"\x1b[A\n")
    session.read_until(b"HACO_HISTORY_COUNT:2")
    session.read_until(prompt_label)

    # Ctrl-C in raw mode must remain terminal input for the guest PTY rather than
    # terminating the local haco client.
    session.send(b"sleep 30\n")
    session.read_until(b"sleep 30")
    time.sleep(0.15)
    session.send(b"\x03")
    session.read_until(prompt_label, timeout=5.0)
    if session.process.poll() is not None:
        session.fail("local haco process exited after terminal Ctrl-C")

    session.send(b"exit\n")
    try:
        return_code = session.process.wait(timeout=10.0)
    except subprocess.TimeoutExpired:
        session.process.kill()
        session.fail("interactive shell did not exit")
    if return_code != 0:
        session.fail(f"interactive shell exited {return_code}, want 0")
    session.assert_restored()


def exercise_termination(session: PTYSession, prompt_label: bytes) -> None:
    session.read_until(prompt_label)
    session.assert_raw()

    session.process.send_signal(signal.SIGTERM)
    try:
        session.process.wait(timeout=5.0)
    except subprocess.TimeoutExpired:
        session.process.kill()
        session.fail("haco did not terminate after SIGTERM")
    session.assert_restored()


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--prompt", required=True, help="plain prompt label to wait for")
    parser.add_argument("--terminate", action="store_true", help="test SIGTERM restoration instead of history")
    parser.add_argument("command", nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command
    if command and command[0] == "--":
        command = command[1:]
    if not command:
        parser.error("command is required after --")

    session = PTYSession(command)
    try:
        if args.terminate:
            exercise_termination(session, args.prompt.encode())
        else:
            exercise_interactive(session, args.prompt.encode())
    finally:
        if session.process.poll() is None:
            session.process.kill()
            try:
                session.process.wait(timeout=2.0)
            except subprocess.TimeoutExpired:
                pass
        session.close()
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # noqa: BLE001 - E2E helper must surface transcript.
        print(f"interactive PTY E2E failed: {exc}", file=sys.stderr)
        raise SystemExit(1)
