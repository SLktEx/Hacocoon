#!/usr/bin/env python3
"""Drive Windows install/reinstall through exact user actions plus read-only assertions.

Product-driving actions are typed exactly as a real user types them. Assertions
run separately and may inspect state, including as root where required, but must
never prepare, repair, restart, remount, attach, detach, create, or delete state.
"""

from __future__ import annotations

import os
import queue
import re
import subprocess
import sys
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Pattern, Sequence

INSTANCE = "Hacocoon"
MANAGED_USER = "hacocoon"
PASSWORD = "Hacocoon-E2E-Only-42!"
ENVIRONMENT = "installer-e2e"
PROJECT = "hacocoon"
POOL = "haco-local-default"
INCUS_POOL_MOUNT = f"/var/lib/incus/storage-pools/{POOL}"
INCUS_BACKING = f"/var/lib/incus/disks/{POOL}.img"
PROCESS_TIMEOUT_SECONDS = 900
ASSERT_TIMEOUT_SECONDS = 120
NATURAL_STOP_TIMEOUT_SECONDS = 90
POST_EXIT_DRAIN_SECONDS = 1.0

TERMINAL_ARGV = ("cmd.exe",)
INSTALL_COMPLETE_RE = re.compile(r"Hacocoon WSL installation complete", re.MULTILINE)

# Only these commands may drive Hacocoon state in the installed user path.
# Read-only assertions live outside this table so the two roles cannot blur.
HOST_SESSION_COMMANDS: dict[str, tuple[str, ...]] = {
    "before reinstall": (
        "haco base list",
        f'haco env create --workspace "$PWD" {ENVIRONMENT}',
        f"haco env status {ENVIRONMENT}",
        f"haco env exec {ENVIRONMENT} -- uname -a",
    ),
    "after reinstall": (
        f"haco env status {ENVIRONMENT}",
        f"haco env exec {ENVIRONMENT} -- uname -a",
        f"haco env delete {ENVIRONMENT}",
    ),
}

ANSI_RE = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]")
OSC_RE = re.compile(r"\x1b\][^\x07]*(?:\x07|\x1b\\)")
CMD_PROMPT_RE = re.compile(r"(?m)^[A-Za-z]:\\[^\r\n>]*>\s*$")


def inherited_child_environment() -> dict[str, str]:
    """Pass the runner environment through unchanged or fail on HACO_* leakage."""

    overrides = sorted(key for key in os.environ if key.startswith("HACO_"))
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


def sudo_responders() -> list[Responder]:
    return [
        responder(r"\[sudo\]\s+password for [^:]+:\s*$", PASSWORD + "\r\n", repeat=True),
        responder(
            r"\[sudo:\s*authenticate\]\s*Password:\s*$",
            PASSWORD + "\r\n",
            repeat=True,
        ),
    ]


def run_bat(package_root: Path, *, phase: str) -> str:
    """ACTION: type the unchanged BAT into a normal cmd.exe prompt.

    ConPTY does not reliably redraw cmd.exe's prompt after a long-running BAT
    until more input arrives. Waiting for that redraw made a successful install
    hang in CI. The installer already prints a normal user-visible completion
    line as its final success signal, so after observing that line we type the
    ordinary outer-shell `exit`. No product command, argument, option, or
    environment is changed.
    """

    process = TerminalProcess(cwd=package_root)
    sent_command = False
    sent_exit = False

    def drive(normalized: str, terminal: TerminalProcess) -> None:
        nonlocal sent_command, sent_exit
        if not sent_command and cmd_prompt_count(normalized):
            terminal.write("install-windows.bat\r\n")
            sent_command = True
            return
        if sent_command and not sent_exit and INSTALL_COMPLETE_RE.search(normalized):
            terminal.write("exit\r\n")
            sent_exit = True

    output = process.run(responders=sudo_responders(), on_output=drive)
    if not sent_command or not sent_exit:
        raise RuntimeError(f"{phase}: install-windows.bat did not reach normal completion")
    require_output(output, r"Hacocoon WSL installation complete", phase=phase)
    return output


def run_host_session(session: str, *, expected_output: list[str]) -> str:
    """ACTION: type normal WSL entry and only ordinary Hacocoon commands."""

    try:
        commands = HOST_SESSION_COMMANDS[session]
    except KeyError as exc:
        raise RuntimeError(f"unknown exact user-path host session: {session!r}") from exc

    process = TerminalProcess()
    sent_wsl = False
    sent_commands = False
    sent_cmd_exit = False
    prompt_before = 0

    def drive(normalized: str, terminal: TerminalProcess) -> None:
        nonlocal sent_wsl, sent_commands, sent_cmd_exit, prompt_before
        prompts = cmd_prompt_count(normalized)
        if not sent_wsl and prompts:
            prompt_before = prompts
            terminal.write("wsl -d Hacocoon\r\n")
            sent_wsl = True
            return
        if sent_wsl and not sent_commands and "haco-host" in normalized:
            for command in commands:
                terminal.write(command + "\r\n")
            terminal.write("exit\r\n")
            sent_commands = True
            return
        if sent_commands and not sent_cmd_exit and prompts > prompt_before:
            terminal.write("exit\r\n")
            sent_cmd_exit = True

    output = process.run(on_output=drive)
    if not sent_wsl or not sent_commands or not sent_cmd_exit:
        raise RuntimeError(f"{session}: exact interactive user session did not complete")
    for pattern in expected_output:
        require_output(output, pattern, phase=session)
    return output


# ---------------------------------------------------------------------------
# Read-only assertion lane. Nothing below is used to drive product state.
# ---------------------------------------------------------------------------


def observe(argv: Sequence[str], *, phase: str) -> str:
    print("==> ASSERT READ-ONLY:", " ".join(argv))
    completed = subprocess.run(
        list(argv),
        env=inherited_child_environment(),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=ASSERT_TIMEOUT_SECONDS,
        check=False,
    )
    stdout = decode_process_output(completed.stdout).strip()
    stderr = decode_process_output(completed.stderr).strip()
    if stdout:
        print(stdout)
    if stderr:
        print(stderr, file=sys.stderr)
    if completed.returncode != 0:
        raise RuntimeError(
            f"{phase}: read-only assertion command failed ({completed.returncode}): "
            + " ".join(argv)
        )
    return stdout


def observe_wsl_root(*args: str, phase: str) -> str:
    return observe(
        ("wsl.exe", "--distribution", INSTANCE, "--user", "root", "--exec", *args),
        phase=phase,
    )


def assert_wsl2(*, phase: str) -> None:
    output = observe(("wsl.exe", "--list", "--verbose"), phase=phase)
    if not re.search(rf"(?im)\b{re.escape(INSTANCE)}\b.*\b2\s*$", output):
        raise RuntimeError(f"{phase}: {INSTANCE} is not listed as WSL 2")


def assert_installed_host_state(*, phase: str) -> None:
    assert_wsl2(phase=phase)

    pid1 = observe_wsl_root("ps", "-p", "1", "-o", "comm=", phase=phase)
    if pid1.strip() != "systemd":
        raise RuntimeError(f"{phase}: PID 1 is {pid1!r}, expected systemd")

    active = observe_wsl_root(
        "systemctl", "is-active", "haco-controller.service", phase=phase
    )
    if active.strip() != "active":
        raise RuntimeError(f"{phase}: haco-controller.service is {active!r}")

    socket = observe_wsl_root(
        "stat", "-Lc", "%U:%G:%a", "/run/hacocoon/control.sock", phase=phase
    )
    if socket.strip() != "root:hacocoon:660":
        raise RuntimeError(f"{phase}: unexpected controller socket state: {socket!r}")

    passwd = observe_wsl_root("getent", "passwd", MANAGED_USER, phase=phase)
    fields = passwd.split(":")
    if len(fields) < 7 or fields[6].strip() != "/usr/local/libexec/hacocoon-login":
        raise RuntimeError(f"{phase}: WSL login integration is incomplete: {passwd!r}")

    groups = observe_wsl_root("id", "-nG", MANAGED_USER, phase=phase).split()
    if "hacocoon" not in groups:
        raise RuntimeError(f"{phase}: managed WSL user lacks hacocoon group: {groups!r}")

    bootstrap = observe_wsl_root(
        "sh",
        "-c",
        "grep -h -F '# BEGIN HACOCOON BOOTSTRAP' /etc/sudoers-rs /etc/sudoers 2>/dev/null || true",
        phase=phase,
    )
    if bootstrap.strip():
        raise RuntimeError(f"{phase}: temporary bootstrap sudo rule remains installed")


def assert_storage_state(*, phase: str) -> None:
    source = observe_wsl_root(
        "incus", "storage", "get", POOL, "source", "--project", PROJECT, phase=phase
    )
    if source.strip() != INCUS_BACKING:
        raise RuntimeError(f"{phase}: pool source is {source!r}, expected {INCUS_BACKING!r}")

    size = observe_wsl_root(
        "incus", "storage", "get", POOL, "size", "--project", PROJECT, phase=phase
    )
    if size.strip() != "128GiB":
        raise RuntimeError(f"{phase}: pool size is {size!r}, expected 128GiB")

    configured = observe_wsl_root(
        "incus",
        "storage",
        "get",
        POOL,
        "btrfs.mount_options",
        "--project",
        PROJECT,
        phase=phase,
    )
    configured_options = {item for item in configured.strip().split(",") if item}
    if "compress=zstd:3" not in configured_options:
        raise RuntimeError(f"{phase}: configured Btrfs options lack zstd: {configured!r}")
    if "autodefrag" in configured_options:
        raise RuntimeError(f"{phase}: autodefrag must remain disabled: {configured!r}")

    fstype = observe_wsl_root(
        "findmnt", "-rn", "-o", "FSTYPE", "--mountpoint", INCUS_POOL_MOUNT, phase=phase
    )
    if fstype.strip() != "btrfs":
        raise RuntimeError(f"{phase}: Incus pool mount is {fstype!r}, expected btrfs")

    live = observe_wsl_root(
        "findmnt", "-rn", "-o", "OPTIONS", "--mountpoint", INCUS_POOL_MOUNT, phase=phase
    )
    live_options = {item for item in live.strip().split(",") if item}
    if not ({"compress=zstd:3", "compress=zstd"} & live_options):
        raise RuntimeError(f"{phase}: live Btrfs mount lacks zstd compression: {live!r}")
    if "autodefrag" in live_options:
        raise RuntimeError(f"{phase}: live Btrfs mount unexpectedly enables autodefrag: {live!r}")

    loop_rows = observe_wsl_root(
        "losetup", "--list", "--noheadings", "--output", "NAME,BACK-FILE", phase=phase
    )
    if INCUS_BACKING not in loop_rows:
        raise RuntimeError(f"{phase}: no loop device backs {INCUS_BACKING}: {loop_rows!r}")

    stat_output = observe_wsl_root("stat", "-Lc", "%s %b", INCUS_BACKING, phase=phase)
    try:
        logical_bytes, allocated_blocks = (int(value) for value in stat_output.split())
    except (ValueError, TypeError) as exc:
        raise RuntimeError(f"{phase}: unexpected backing stat output: {stat_output!r}") from exc
    allocated_bytes = allocated_blocks * 512
    if logical_bytes != 128 * 1024 * 1024 * 1024:
        raise RuntimeError(f"{phase}: backing logical size is {logical_bytes}")
    if allocated_bytes >= logical_bytes:
        raise RuntimeError(
            f"{phase}: backing image is not sparse: allocated={allocated_bytes} logical={logical_bytes}"
        )


def assert_environment_runtime(*, present: bool, phase: str) -> None:
    rows = observe_wsl_root(
        "incus", "list", "--project", PROJECT, "--format", "csv", "-c", "n,s", phase=phase
    )
    expected_name = f"haco-{ENVIRONMENT}"
    matching = [line for line in rows.splitlines() if line.split(",", 1)[0] == expected_name]
    if present:
        if not matching or not any(line.endswith(",RUNNING") for line in matching):
            raise RuntimeError(
                f"{phase}: expected running Environment runtime {expected_name!r}: {rows!r}"
            )
    elif matching:
        raise RuntimeError(f"{phase}: deleted Environment runtime remains: {matching!r}")


def wait_for_natural_wsl_stop(*, phase: str) -> None:
    """Observe WSL's normal idle lifecycle without injecting terminate/shutdown."""

    deadline = time.monotonic() + NATURAL_STOP_TIMEOUT_SECONDS
    while time.monotonic() < deadline:
        running = observe(("wsl.exe", "--list", "--running"), phase=phase)
        if not re.search(rf"(?im)\b{re.escape(INSTANCE)}\b", running):
            print(f"==> ASSERT READ-ONLY: {INSTANCE} stopped naturally")
            return
        time.sleep(2)
    raise RuntimeError(
        f"{phase}: {INSTANCE} did not reach normal stopped state within "
        f"{NATURAL_STOP_TIMEOUT_SECONDS}s"
    )


def main() -> int:
    if os.name != "nt":
        raise RuntimeError("Windows user-path E2E must run on Windows")
    inherited_child_environment()

    package_root = Path.cwd()
    required = (
        "install-windows.bat",
        "install-windows.ps1",
        "install.sh",
        "haco_linux_amd64.tar.gz",
        "checksums.txt",
        "VERSION",
    )
    missing = [name for name in required if not (package_root / name).is_file()]
    if missing:
        raise RuntimeError(f"run from extracted Windows package; missing: {', '.join(missing)}")

    print("==> ACTION USER TYPES: install-windows.bat")
    first = run_bat(package_root, phase="first install")
    require_output(first, r"Hacocoon WSL installation complete", phase="first install")
    assert_installed_host_state(phase="after first BAT")

    print("==> ACTION USER TYPES: wsl -d Hacocoon and normal haco commands")
    run_host_session(
        "before reinstall",
        expected_output=[
            r"(?m)^haco/ubuntu-26\.04\s*$",
            rf"(?m)^name:\s+{re.escape(ENVIRONMENT)}\s*$",
            r"(?m)^Linux\s+",
        ],
    )
    assert_environment_runtime(present=True, phase="after Environment create")
    assert_storage_state(phase="after Environment create")

    # A user can close WSL and rerun the installer later. Observe the natural
    # idle shutdown instead of manufacturing it with `wsl --terminate`.
    wait_for_natural_wsl_stop(phase="before reinstall")

    print("==> ACTION USER TYPES: install-windows.bat")
    second = run_bat(package_root, phase="reinstall")
    require_output(second, r"Hacocoon WSL installation complete", phase="reinstall")
    assert_installed_host_state(phase="after reinstall")
    assert_environment_runtime(present=True, phase="after reinstall")
    assert_storage_state(phase="after reinstall")

    print("==> ACTION USER TYPES: wsl -d Hacocoon and reuse existing Environment")
    run_host_session(
        "after reinstall",
        expected_output=[
            rf"(?m)^name:\s+{re.escape(ENVIRONMENT)}\s*$",
            r"(?m)^Linux\s+",
        ],
    )
    assert_environment_runtime(present=False, phase="after Environment delete")
    assert_storage_state(phase="after Environment delete")

    print("windows installer exact user actions + read-only assertions: PASS")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(
            f"windows installer exact user actions + read-only assertions: FAIL: {exc}",
            file=sys.stderr,
        )
        raise
