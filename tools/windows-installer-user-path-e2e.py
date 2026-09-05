#!/usr/bin/env python3
"""Drive Windows install/reinstall through exact user actions plus read-only assertions.

Product-driving actions are typed exactly as a real user types them. Assertions
run separately and may inspect state, including as root where required, but must
never prepare, repair, restart, remount, attach, detach, create, or delete state.
"""

from __future__ import annotations

import os
import queue
import secrets
import re
import subprocess
import sys
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Pattern, Sequence

INSTANCE = "Hacocoon"
LOGIN_USER = "haco-e2e"
PASSWORD = secrets.token_urlsafe(24)
SENTINEL = ".hacocoon-installer-acceptance"
PROJECT = "hacocoon"
POOL = "haco-local-default"
INCUS_POOL_MOUNT = f"/var/lib/incus/storage-pools/{POOL}"
INCUS_BACKING = f"/var/lib/incus/disks/{POOL}.img"
PROCESS_TIMEOUT_SECONDS = 900
ASSERT_TIMEOUT_SECONDS = 120

POST_EXIT_DRAIN_SECONDS = 1.0

# GitHub's Windows Python can expose a cp1252 text stream when PowerShell
# redirects this harness to a log file. Product output legitimately contains
# Unicode (for example arrows). Logging must never abort the user journey merely
# because the outer CI stream cannot encode one character. This changes only the
# harness's own display error handling; child process environment/input is untouched.
for _stream in (sys.stdout, sys.stderr):
    if hasattr(_stream, "reconfigure"):
        _stream.reconfigure(errors="replace")

TERMINAL_ARGV = ("cmd.exe",)
INSTALL_COMPLETE_RE = re.compile(r"Hacocoon Windows installation complete\.", re.MULTILINE)

ANSI_RE = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]")
OSC_RE = re.compile(r"\x1b\][^\x07]*?(?:\x07|\x1b\\)")
CMD_PROMPT_RE = re.compile(r"(?m)^[A-Za-z]:\\[^\r\n>]*>\s*$")


def inherited_child_environment() -> dict[str, str]:
    """Pass the runner environment through unchanged or fail on HACO_* leakage."""

    overrides = sorted(key for key in os.environ if key.startswith(("HACO_", "HACOQ_", "HACOHOST_")))
    if overrides:
        raise RuntimeError(
            "exact user-path E2E refuses Hacocoon environment overrides: "
            + ", ".join(overrides)
        )
    return dict(os.environ)


def normalize_terminal(text: str) -> str:
    return OSC_RE.sub("", ANSI_RE.sub("", text))


def cmd_prompt_count(text: str) -> int:
    return len(CMD_PROMPT_RE.findall(text))


def decode_process_output(data: bytes) -> str:
    if not data:
        return ""
    # Redirected `wsl --list` may be UTF-16LE; Linux command output is UTF-8.
    if data.startswith(b"\xff\xfe") or b"\x00" in data[:80]:
        return data.decode("utf-16-le", errors="replace").lstrip("\ufeff")
    return data.decode("utf-8", errors="replace")


@dataclass
class Responder:
    pattern: Pattern[str]
    reply: str
    repeat: bool = False
    fired: int = 0


def responder(pattern: str, reply: str, *, repeat: bool = False) -> Responder:
    return Responder(re.compile(pattern, re.IGNORECASE | re.MULTILINE), reply, repeat)


class TerminalProcess:
    def __init__(self, *, cwd: Path | None = None):
        try:
            from winpty import PtyProcess
        except ImportError as exc:
            raise RuntimeError("pywinpty is required for the Windows user-path E2E") from exc

        self.proc = PtyProcess.spawn(
            list(TERMINAL_ARGV),
            cwd=str(cwd) if cwd is not None else None,
            env=inherited_child_environment(),
            dimensions=(48, 160),
        )
        self.output = ""
        self._queue: queue.Queue[str | None] = queue.Queue()
        self._reader = threading.Thread(target=self._read_loop, daemon=True)
        self._reader.start()

    def _read_loop(self) -> None:
        while True:
            try:
                chunk = self.proc.read(4096)
            except EOFError:
                self._queue.put(None)
                return
            except Exception as exc:  # pragma: no cover - runtime diagnostic
                self._queue.put(f"\n[terminal reader error: {exc}]\n")
                self._queue.put(None)
                return
            if chunk:
                self._queue.put(chunk)

    def write(self, text: str) -> None:
        self.proc.write(text)

    def _consume(
        self,
        chunk: str,
        responders: list[Responder],
        on_output: Callable[[str, "TerminalProcess"], None] | None,
    ) -> None:
        sys.stdout.write(chunk)
        sys.stdout.flush()
        self.output += chunk
        normalized = normalize_terminal(self.output)
        for item in responders:
            if item.fired and not item.repeat:
                continue
            matches = list(item.pattern.finditer(normalized))
            if len(matches) > item.fired:
                item.fired += 1
                self.write(item.reply)
        if on_output is not None:
            on_output(normalized, self)

    def run(
        self,
        *,
        responders: list[Responder] | None = None,
        on_output: Callable[[str, "TerminalProcess"], None] | None = None,
        timeout: int = PROCESS_TIMEOUT_SECONDS,
    ) -> str:
        responders = responders or []
        deadline = time.monotonic() + timeout
        dead_since: float | None = None

        while time.monotonic() < deadline:
            try:
                chunk = self._queue.get(timeout=0.1)
            except queue.Empty:
                chunk = ""

            if chunk is None:
                break
            if chunk:
                dead_since = None
                self._consume(chunk, responders, on_output)
                continue
            if self.proc.isalive():
                dead_since = None
                continue
            if dead_since is None:
                dead_since = time.monotonic()
                continue
            if time.monotonic() - dead_since < POST_EXIT_DRAIN_SECONDS:
                continue
            while True:
                try:
                    pending = self._queue.get_nowait()
                except queue.Empty:
                    break
                if pending:
                    self._consume(pending, responders, on_output)
            break
        else:
            self.proc.terminate(force=True)
            raise RuntimeError("timed out waiting for exact user terminal session")

        exit_status = self.proc.exitstatus
        if exit_status not in (None, 0):
            raise RuntimeError(f"terminal session failed with exit status {exit_status}")
        return normalize_terminal(self.output)


def require_output(output: str, pattern: str, *, phase: str) -> None:
    if not re.search(pattern, output, re.MULTILINE):
        raise RuntimeError(f"{phase}: expected user-visible output matching {pattern!r}")


def observe(*args: str) -> str:
    result = subprocess.run(args, env=inherited_child_environment(), capture_output=True,
                            timeout=ASSERT_TIMEOUT_SECONDS, check=True)
    return decode_process_output(result.stdout).strip()


def inspect_root(*args: str) -> str:
    return observe("wsl.exe", "-d", INSTANCE, "-u", "root", "--exec", *args)


def run_bat(package_root: Path) -> None:
    terminal = TerminalProcess(cwd=package_root)
    sent_bat = sent_exit = False

    def drive(output: str, process: TerminalProcess) -> None:
        nonlocal sent_bat, sent_exit
        if not sent_bat and cmd_prompt_count(output):
            process.write("install-windows.bat\r\n")
            sent_bat = True
        elif sent_bat and not sent_exit and INSTALL_COMPLETE_RE.search(output):
            process.write("exit\r\n")
            sent_exit = True

    # Ubuntu's own dialogs, via keystrokes. Never useradd, passwd, sudoers,
    # cloud-init fixtures, product overrides, or modifications to distro OOBE.
    responses = [
        responder(r"Create a default Unix user account:[^\r\n]*$", "\x15" + LOGIN_USER + "\r\n"),
        responder(r"(?<!Retype )(?<!retype )New password:\s*$", PASSWORD + "\r\n"),
        responder(r"Retype new password:\s*$", PASSWORD + "\r\n"),
        responder(r"^\[Y/n/e\]:[^\r\n]*$", "\x15n\r\n"),
        responder(r"(?m)^[^\r\n]*@[^\r\n]*:[^\r\n]*\$\s*$", "exit\r\n"),
    ]
    output = terminal.run(responders=responses, on_output=drive)
    if not sent_bat or not sent_exit:
        raise RuntimeError("BAT did not complete; no second BAT may repair first-install acceptance")
    require_output(output, r"Hacocoon WSL installation complete", phase="BAT")


def host_session(*, create: bool) -> None:
    terminal = TerminalProcess()
    stage = 0
    sent_at = 0
    # Only the currently implemented product CLI is used. Environment/SSH
    # commands remain a separate gate until the reset CLI implements them.
    commands = ["haco version --json", "haco help"]
    if create:
        commands.append(f"printf '%s\\n' kept-through-restart-and-rerun > ~/{SENTINEL}")
    commands += [f"cat ~/{SENTINEL}", "exit"]

    def drive(output: str, process: TerminalProcess) -> None:
        nonlocal stage, sent_at
        if stage == 0 and cmd_prompt_count(output):
            process.write("wsl -d Hacocoon\r\n")
            stage, sent_at = 1, len(output)
        elif stage == 1 and re.search(r"(?m)^[^\r\n]*@haco-host:[^\r\n]*[#\$]\s*$", output[sent_at:]):
            process.write("\r\n".join(commands) + "\r\n")
            stage, sent_at = 2, len(output)
        elif stage == 2 and cmd_prompt_count(output[sent_at:]):
            process.write("exit\r\n")
            stage = 3

    output = terminal.run(on_output=drive)
    if stage != 3:
        raise RuntimeError("ordinary WSL entry did not reach haco-host and return normally")
    require_output(output, r"(?m)^kept-through-restart-and-rerun\s*$", phase="haco-host data")
    require_output(output, r'"version"\s*:', phase="product CLI")
    if re.search(r"command not found|unknown command|permission denied", output, re.I):
        raise RuntimeError("ordinary host commands failed")


def assert_host() -> None:
    if inspect_root("ps", "-p", "1", "-o", "comm=") != "systemd":
        raise RuntimeError("systemd is not PID 1")
    inspect_root("systemctl", "is-active", "--quiet", "haco-controller.service")
    group = inspect_root("getent", "group", "hacocoon").split(":")
    if len(group) != 4 or not group[2].isdigit() or group[2] == "0":
        raise RuntimeError("invalid controller access group")
    if inspect_root("stat", "-Lc", "%u:%g:%a", "/run/hacocoon/control.sock") != f"0:{group[2]}:660":
        raise RuntimeError("unsafe Physical Host controller socket")
    if observe("wsl.exe", "-d", INSTANCE, "--exec", "id", "-un") != LOGIN_USER:
        raise RuntimeError("ordinary WSL login identity changed")
    if not inspect_root("getent", "passwd", LOGIN_USER).endswith(":/usr/local/libexec/hacocoon-login"):
        raise RuntimeError("WSL login integration missing")
    inspect_root("incus", "exec", "haco-host", "--project", PROJECT, "--", "/usr/local/bin/haco-host", "doctor")
    if inspect_root("incus", "storage", "get", POOL, "btrfs.mount_options") != "compress=zstd:3,noatime,nodiscard":
        raise RuntimeError("Incus storage mount policy drift")
    if inspect_root("incus", "storage", "get", POOL, "source") != INCUS_BACKING:
        raise RuntimeError("storage is not owned by Incus")
    if inspect_root("findmnt", "-rn", "-o", "FSTYPE", "--mountpoint", INCUS_POOL_MOUNT) != "btrfs":
        raise RuntimeError("Incus pool is not mounted as Btrfs")


def sudo_policy_digest() -> str:
    return inspect_root("sh", "-eu", "-c",
                        "find /etc -maxdepth 2 -type f -name '*sudoers*' -exec sha256sum {} + | sort")


def main() -> None:
    if os.name != "nt":
        raise RuntimeError("this gate requires Windows")
    inherited_child_environment()
    existing = observe("wsl.exe", "--list", "--quiet").splitlines()
    if INSTANCE.casefold() in {name.strip().casefold() for name in existing}:
        raise RuntimeError("fresh-install gate refuses an existing Hacocoon distribution")
    package_root = Path.cwd()
    if not (package_root / "install-windows.bat").is_file():
        raise RuntimeError("run from the extracted candidate ZIP")
    run_bat(package_root)
    assert_host()
    host_session(create=True)
    # A normal user stop, before any installer rerun that could repair startup.
    subprocess.run(["wsl.exe", "--terminate", INSTANCE], check=True, timeout=120)
    host_session(create=False)
    assert_host()
    policy_before = sudo_policy_digest()
    run_bat(package_root)
    host_session(create=False)
    assert_host()
    if sudo_policy_digest() != policy_before:
        raise RuntimeError("current-installer rerun changed existing sudo policy")
    print("Windows BAT / WSL entry / restart / trusted-host data retention: PASS")


if __name__ == "__main__":
    main()
