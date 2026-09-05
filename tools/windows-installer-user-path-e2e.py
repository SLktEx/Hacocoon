#!/usr/bin/env python3
"""Drive Windows install/reinstall through exact user actions plus read-only assertions.

Product-driving actions are typed exactly as a real user types them. Assertions
run separately and may inspect state, including as root where required, but must
never prepare, repair, restart, remount, attach, detach, create, or delete state.
"""

from __future__ import annotations

import json
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
LOGIN_USER = "hacocoon"
PASSWORD = secrets.token_urlsafe(24)
SENTINEL = ".hacocoon-installer-acceptance"
PROJECT = "hacocoon"
POOL = "haco-local-default"
INCUS_POOL_MOUNT = f"/var/lib/incus/storage-pools/{POOL}"
INCUS_BACKING = f"/var/lib/incus/disks/{POOL}.img"
PROCESS_TIMEOUT_SECONDS = 1800
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
    # ConPTY can insert a bare CR after CRLF and before an OSC-wrapped result.
    # Drop this cursor control without inventing a new line: anchored assertions
    # must still distinguish command output from an echoed command.
    return OSC_RE.sub("", ANSI_RE.sub("", text)).replace("\r", "")


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
    commands = ["haco version --json", "haco help", "haco doctor --json && printf '%s\\n' HACO_DOCTOR_OK"]
    # Exercise the installer-created trusted-host network in the ordinary
    # shell. This is infrastructure egress, not Environment proxy acceptance.
    commands += [
        "getent ahostsv4 github.com >/dev/null && printf '%s\\n' HACO_HOST_DNS_OK",
        "ip -4 route show default | grep -q '^default ' && printf '%s\\n' HACO_HOST_ROUTE_OK",
        "curl -4 -f -sS --connect-timeout 10 --max-time 30 -o /dev/null https://github.com && printf '%s\\n' HACO_HOST_HTTPS_OK",
    ]
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
    require_output(output, r"^HACO_DOCTOR_OK\s*$", phase="product doctor")
    for check in ("DNS", "ROUTE", "HTTPS"):
        require_output(output, rf"^HACO_HOST_{check}_OK\s*$", phase="trusted-host network")
    if re.search(r"command not found|unknown command|permission denied", output, re.I):
        raise RuntimeError("ordinary host commands failed")


def assert_doctor_report(output: str, expected_build: dict[str, str]) -> None:
    report = json.loads(output)
    names = ["runtime", "storage", "trusted_host", "trusted_network", "trusted_connectivity"]
    fields = ("checkpoint", "version", "commit", "build_date")
    if any(expected_build.get(field) in (None, "", "dev", "unknown") for field in fields):
        raise RuntimeError("packaged client has incomplete build identity")
    if report["protocol_version"] != 1 or report["controller"] != expected_build:
        raise RuntimeError("doctor controller build does not match the packaged client")
    if [check["name"] for check in report["checks"]] != names or any(check["status"] != "ok" for check in report["checks"]):
        raise RuntimeError("doctor checks did not all pass")


def assert_host() -> None:
    expected_build = json.loads(observe("wsl.exe", "-d", INSTANCE, "--exec", "haco", "version", "--json"))
    assert_doctor_report(observe("wsl.exe", "-d", INSTANCE, "--exec", "haco", "doctor", "--json"), expected_build)
    assert_doctor_report(inspect_root("incus", "exec", "haco-host", "--project", PROJECT, "--", "/usr/local/bin/haco", "doctor", "--json"), expected_build)
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
    trusted_host = json.loads(inspect_root("incus", "query", f"/1.0/instances/haco-host?project={PROJECT}"))
    if trusted_host["profiles"] or trusted_host["devices"].get("eth0") != {
        "type": "nic", "name": "eth0", "network": "haco-host0",
    }:
        raise RuntimeError("trusted host inherited a profile or uses an unexpected network")
    if inspect_root("incus", "network", "get", "haco-host0", "user.hacocoon.owner", "--project", "default") != "trusted-host-network-v1":
        raise RuntimeError("trusted-host network ownership mismatch")
    # This gate starts with an absent distro. A fresh product install creates
    # only the Incus Btrfs pool; current-data migration is a separate check.
    if set(inspect_root("incus", "storage", "list", "--format", "csv", "-c", "n").splitlines()) != {POOL}:
        raise RuntimeError("fresh installer created an additional storage pool")
    if inspect_root("incus", "storage", "get", POOL, "btrfs.mount_options") != "compress=zstd:3,noatime,nodiscard":
        raise RuntimeError("Incus storage mount policy drift")
    if inspect_root("incus", "storage", "get", POOL, "source") != INCUS_BACKING:
        raise RuntimeError("storage is not owned by Incus")
    if inspect_root("findmnt", "-rn", "-o", "FSTYPE", "--mountpoint", INCUS_POOL_MOUNT) != "btrfs":
        raise RuntimeError("Incus pool is not mounted as Btrfs")
    inspect_root("sh", "-eu", "-c",
                 "getent ahostsv4 github.com >/dev/null; "
                 "ip -4 route show default | grep -q '^default '; "
                 "curl -4 -f -sS --connect-timeout 10 --max-time 30 -o /dev/null https://github.com")


def sudo_policy_digest() -> str:
    return inspect_root("sh", "-eu", "-c",
                        "find /etc/sudoers /etc/sudoers.d -type f -exec sha256sum {} + | sort")


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
    # Exercise the documented direct diagnostic entry before any root/service
    # observation or interactive login can hide cold controller startup.
    expected_build = json.loads(observe("wsl.exe", "-d", INSTANCE, "--exec", "haco", "version", "--json"))
    subprocess.run(["wsl.exe", "--terminate", INSTANCE], check=True, timeout=120)
    assert_doctor_report(observe("wsl.exe", "-d", INSTANCE, "--exec", "haco", "doctor", "--json"), expected_build)
    print("Windows BAT / WSL entry / restart / trusted-host data retention / cold doctor: PASS")


if __name__ == "__main__":
    main()
