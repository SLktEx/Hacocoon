#!/usr/bin/env python3
"""Drive Windows install/reinstall through the real user path.

Product actions use the same command line and interaction a real user uses.
The harness must not add HACO_* overrides, installer-only arguments/options,
CI sudo rules, root preparation, or state repair to make an action succeed.

Read-only assertions may inspect the state produced by an action. A deliberate
`wsl --terminate Hacocoon` is a lifecycle fault injection that reproduces a
normal WSL/Host restart; nothing repairs Hacocoon state before the unchanged
installer is run again.
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
from typing import Callable, Pattern

INSTANCE = "Hacocoon"
PASSWORD = "Hacocoon-E2E-Only-42!"
ENVIRONMENT = "installer-e2e"
ENVIRONMENT_INSTANCE = f"haco-{ENVIRONMENT}"
WORKSPACE = "~/installer-e2e-workspace"
PROJECT = "hacocoon"
POOL = "haco-local-default"
MOUNTPOINT = "/var/lib/hacocoon/mounts/local-default"
BACKING = "/var/lib/hacocoon/images/local-default.raw"
PROCESS_TIMEOUT_SECONDS = 900
POST_EXIT_DRAIN_SECONDS = 1.0

ANSI_RE = re.compile(r"\x1b\[[0-?]*[ -/]*[@-~]")
OSC_RE = re.compile(r"\x1b\][^\x07]*(?:\x07|\x1b\\)")


def inherited_child_environment() -> dict[str, str]:
    overrides = sorted(key for key in os.environ if key.startswith("HACO_"))
    if overrides:
        raise RuntimeError(
            "exact user-path E2E refuses Hacocoon environment overrides: "
            + ", ".join(overrides)
        )
    return dict(os.environ)


def normal_user_name() -> str:
    value = os.environ.get("USERNAME", "runneradmin").lower().replace(" ", "_")
    value = re.sub(r"[^a-z0-9_-]", "", value)
    if not value or not re.match(r"^[a-z_]", value):
        value = "runneradmin"
    return value[:32]


def normalize_terminal(text: str) -> str:
    return OSC_RE.sub("", ANSI_RE.sub("", text))


@dataclass
class Responder:
    pattern: Pattern[str]
    reply: str
    repeat: bool = False
    fired: int = 0


def responder(pattern: str, reply: str, *, repeat: bool = False) -> Responder:
    return Responder(re.compile(pattern, re.IGNORECASE | re.MULTILINE), reply, repeat)


class TerminalProcess:
    def __init__(self, argv: list[str], *, cwd: Path | None = None):
        try:
            from winpty import PtyProcess
        except ImportError as exc:
            raise RuntimeError("pywinpty is required for the Windows user-path E2E") from exc

        self.argv = argv
        self.proc = PtyProcess.spawn(
            argv,
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
            raise RuntimeError(f"timed out waiting for terminal command: {self.argv!r}")

        exit_status = self.proc.exitstatus
        if exit_status not in (None, 0):
            raise RuntimeError(
                f"terminal command failed with exit status {exit_status}: {self.argv!r}"
            )
        return normalize_terminal(self.output)


def run_bat(package_root: Path, *, phase: str) -> str:
    # ACT: this is exactly the user-facing installer command. The cmd.exe flags
    # only launch a BAT non-interactively; install-windows.bat receives no args.
    process = TerminalProcess(
        ["cmd.exe", "/d", "/c", "install-windows.bat"],
        cwd=package_root,
    )
    output = process.run(
        responders=[
            responder(r"\[sudo\]\s+password for [^:]+:\s*$", PASSWORD + "\r\n", repeat=True),
        ]
    )
    if "Hacocoon" not in output:
        raise RuntimeError(f"{phase}: installer produced no Hacocoon output")
    return output


def complete_ubuntu_first_launch() -> str:
    # ACT: documented first-launch command.
    process = TerminalProcess(["wsl.exe", "-d", INSTANCE])
    user = normal_user_name()
    sent_exit = False

    def maybe_exit(normalized: str, terminal: TerminalProcess) -> None:
        nonlocal sent_exit
        if sent_exit:
            return
        prompt = rf"{re.escape(user)}@[^:\r\n]+:[^\r\n]*\$\s*$"
        if re.search(prompt, normalized, re.MULTILINE):
            terminal.write("exit\r\n")
            sent_exit = True

    output = process.run(
        responders=[
            responder(r"Create a default Unix user account:\s*", "\r\n"),
            responder(r"New password:\s*$", PASSWORD + "\r\n"),
            responder(r"Retype new password:\s*$", PASSWORD + "\r\n"),
            responder(
                r"(?:Would you like to opt-in to platform metrics collection|\[Y/n/e\]:\s*)",
                "\r\n",
            ),
        ],
        on_output=maybe_exit,
    )
    if not sent_exit:
        raise RuntimeError("Ubuntu first-launch completed without reaching the normal user shell")
    return output


def run_host_session(commands: list[str], *, expected_markers: list[str]) -> str:
    # ACT transport: documented WSL entry. Commands supplied here use normal
    # shell/Hacocoon syntax. printf markers are read-only assertions only.
    process = TerminalProcess(["wsl.exe", "-d", INSTANCE])
    sent = False

    def send_commands(normalized: str, terminal: TerminalProcess) -> None:
        nonlocal sent
        if sent or "haco-host" not in normalized:
            return
        for command in commands:
            terminal.write(command + "\r\n")
        terminal.write("exit\r\n")
        sent = True

    output = process.run(on_output=send_commands)
    if not sent:
        raise RuntimeError("interactive WSL entry never reached haco-host")
    for marker in expected_markers:
        if marker not in output:
            raise RuntimeError(f"missing user-path assertion marker: {marker}")
    return output


def run_readonly_wsl(argv: list[str], *, phase: str) -> str:
    # ASSERT only. This direct root path is allowed because it reads state and
    # cannot prepare or repair the next product action.
    completed = subprocess.run(
        [
            "wsl.exe",
            "--distribution",
            INSTANCE,
            "--user",
            "root",
            "--exec",
            *argv,
        ],
        text=True,
        capture_output=True,
        check=False,
        env=inherited_child_environment(),
    )
    if completed.stdout:
        print(completed.stdout, end="" if completed.stdout.endswith("\n") else "\n")
    if completed.stderr:
        print(completed.stderr, file=sys.stderr, end="" if completed.stderr.endswith("\n") else "\n")
    if completed.returncode != 0:
        raise RuntimeError(
            f"{phase}: read-only assertion failed ({completed.returncode}): {argv!r}"
        )
    return completed.stdout.strip()


def assert_managed_storage(*, phase: str) -> None:
    print(f"==> ASSERT: managed storage ({phase})")

    service = run_readonly_wsl(
        ["systemctl", "is-active", "haco-controller.service"], phase=phase
    )
    if service != "active":
        raise RuntimeError(f"{phase}: haco-controller is not active: {service!r}")

    pool_source = run_readonly_wsl(
        ["incus", "storage", "get", POOL, "source", "--project", PROJECT],
        phase=phase,
    )
    if pool_source != MOUNTPOINT:
        raise RuntimeError(
            f"{phase}: Incus pool source is {pool_source!r}, expected {MOUNTPOINT!r}"
        )

    pool_list = run_readonly_wsl(
        ["incus", "storage", "list", "--project", PROJECT], phase=phase
    )
    if POOL not in pool_list:
        raise RuntimeError(f"{phase}: managed Incus storage pool is missing")
    if "UNAVAILABLE" in pool_list.upper():
        raise RuntimeError(f"{phase}: managed Incus storage pool is unavailable:\n{pool_list}")

    fstype = run_readonly_wsl(
        ["findmnt", "-rn", "-o", "FSTYPE", "--mountpoint", MOUNTPOINT], phase=phase
    )
    if fstype != "btrfs":
        raise RuntimeError(f"{phase}: managed mount fstype is {fstype!r}, expected 'btrfs'")

    options = run_readonly_wsl(
        ["findmnt", "-rn", "-o", "OPTIONS", "--mountpoint", MOUNTPOINT], phase=phase
    )
    if not re.search(r"(?:^|,)compress=zstd(?::3)?(?:,|$)", options):
        raise RuntimeError(f"{phase}: managed mount is missing zstd compression: {options}")

    source = run_readonly_wsl(
        ["findmnt", "-rn", "-o", "SOURCE", "--mountpoint", MOUNTPOINT], phase=phase
    )
    if not re.fullmatch(r"/dev/loop\d+", source):
        raise RuntimeError(f"{phase}: managed mount has unexpected source: {source!r}")

    backing_type = run_readonly_wsl(["stat", "-Lc", "%F", BACKING], phase=phase)
    if backing_type != "regular file":
        raise RuntimeError(f"{phase}: managed backing is not a regular file: {backing_type!r}")

    loop_rows = run_readonly_wsl(
        ["losetup", "--list", "--noheadings", "--output", "NAME,BACK-FILE"], phase=phase
    )
    if not any(
        len(fields := row.split(None, 1)) == 2
        and fields[0] == source
        and fields[1] == BACKING
        for row in loop_rows.splitlines()
    ):
        raise RuntimeError(f"{phase}: {source} is not attached to {BACKING}")

    incus_pid = run_readonly_wsl(
        ["systemctl", "show", "--property", "MainPID", "--value", "incus.service"],
        phase=phase,
    )
    if not incus_pid.isdigit() or int(incus_pid) <= 1:
        raise RuntimeError(f"{phase}: incus.service has invalid MainPID: {incus_pid!r}")
    incus_fstype = run_readonly_wsl(
        [
            "nsenter",
            f"--mount=/proc/{incus_pid}/ns/mnt",
            "--",
            "findmnt",
            "-rn",
            "-o",
            "FSTYPE",
            "--mountpoint",
            MOUNTPOINT,
        ],
        phase=phase,
    )
    if incus_fstype != "btrfs":
        raise RuntimeError(
            f"{phase}: managed Btrfs mount is not visible in incusd namespace: {incus_fstype!r}"
        )


def assert_environment_running(*, phase: str) -> None:
    print(f"==> ASSERT: Environment running ({phase})")
    row = run_readonly_wsl(
        [
            "incus",
            "list",
            ENVIRONMENT_INSTANCE,
            "--project",
            PROJECT,
            "--format",
            "csv",
            "-c",
            "n,s",
        ],
        phase=phase,
    )
    expected = f"{ENVIRONMENT_INSTANCE},RUNNING"
    if row != expected:
        raise RuntimeError(f"{phase}: Environment row is {row!r}, expected {expected!r}")


def assert_environment_deleted(*, phase: str) -> None:
    print(f"==> ASSERT: Environment deleted ({phase})")
    row = run_readonly_wsl(
        [
            "incus",
            "list",
            ENVIRONMENT_INSTANCE,
            "--project",
            PROJECT,
            "--format",
            "csv",
            "-c",
            "n",
        ],
        phase=phase,
    )
    if row:
        raise RuntimeError(f"{phase}: Environment instance still exists: {row!r}")


def terminate_wsl_for_restart() -> None:
    # LIFECYCLE INJECTION, not preparation. This intentionally discards WSL
    # runtime namespaces/mounts so the next unchanged BAT must recover them.
    print("==> LIFECYCLE: terminate Hacocoon WSL to reproduce a real restart")
    completed = subprocess.run(
        ["wsl.exe", "--terminate", INSTANCE],
        check=False,
        env=inherited_child_environment(),
    )
    if completed.returncode != 0:
        raise RuntimeError("failed to terminate Hacocoon WSL for restart acceptance")
    time.sleep(0.75)


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

    print("==> ACT: first install-windows.bat")
    first = run_bat(package_root, phase="phase 1")
    if "wsl -d Hacocoon" not in first:
        raise RuntimeError("phase 1 did not request normal WSL first launch")

    print("==> ACT: wsl -d Hacocoon first-launch OOBE")
    complete_ubuntu_first_launch()

    print("==> ACT: second install-windows.bat")
    second = run_bat(package_root, phase="phase 2")
    if "Hacocoon WSL installation complete" not in second:
        raise RuntimeError("phase 2 did not complete the packaged installation")

    assert_managed_storage(phase="after initial install")

    print("==> ACT: wsl -d Hacocoon; create and use a live Environment")
    run_host_session(
        [
            "haco version",
            "printf '__ASSERT_VERSION_RC__:%s\\n' \"$?\"",
            "haco doctor",
            "printf '__ASSERT_DOCTOR_BEFORE_RC__:%s\\n' \"$?\"",
            f"mkdir -p {WORKSPACE}",
            "printf 'before-reinstall\\n' > ~/installer-e2e-workspace/input.txt",
            f"haco env create --workspace {WORKSPACE} {ENVIRONMENT}",
            "printf '__ASSERT_CREATE_RC__:%s\\n' \"$?\"",
            f"haco env status {ENVIRONMENT}",
            "printf '__ASSERT_STATUS_BEFORE_RC__:%s\\n' \"$?\"",
            f"haco env exec {ENVIRONMENT} -- cat /workspace/input.txt",
            "printf '__ASSERT_EXEC_BEFORE_RC__:%s\\n' \"$?\"",
        ],
        expected_markers=[
            "__ASSERT_VERSION_RC__:0",
            "__ASSERT_DOCTOR_BEFORE_RC__:0",
            "__ASSERT_CREATE_RC__:0",
            "__ASSERT_STATUS_BEFORE_RC__:0",
            "before-reinstall",
            "__ASSERT_EXEC_BEFORE_RC__:0",
        ],
    )
    assert_environment_running(phase="before WSL restart")
    assert_managed_storage(phase="before WSL restart")

    terminate_wsl_for_restart()

    # Nothing is asserted, mounted, attached, started, or repaired between the
    # terminate and this ACT. The ordinary installer owns all reconciliation.
    print("==> ACT: rerun unchanged install-windows.bat after WSL restart")
    third = run_bat(package_root, phase="restart/reinstall")
    if "Hacocoon WSL installation complete" not in third:
        raise RuntimeError("reinstall did not complete after WSL restart")

    assert_managed_storage(phase="after WSL restart/reinstall")
    assert_environment_running(phase="after WSL restart/reinstall")

    print("==> ACT: wsl -d Hacocoon; verify the existing Environment")
    run_host_session(
        [
            "haco doctor",
            "printf '__ASSERT_DOCTOR_AFTER_RC__:%s\\n' \"$?\"",
            f"haco env status {ENVIRONMENT}",
            "printf '__ASSERT_STATUS_AFTER_RC__:%s\\n' \"$?\"",
            f"haco env exec {ENVIRONMENT} -- cat /workspace/input.txt",
            "printf '__ASSERT_EXEC_AFTER_RC__:%s\\n' \"$?\"",
            f"haco env delete {ENVIRONMENT}",
            "printf '__ASSERT_DELETE_RC__:%s\\n' \"$?\"",
        ],
        expected_markers=[
            "__ASSERT_DOCTOR_AFTER_RC__:0",
            "__ASSERT_STATUS_AFTER_RC__:0",
            "before-reinstall",
            "__ASSERT_EXEC_AFTER_RC__:0",
            "__ASSERT_DELETE_RC__:0",
        ],
    )
    assert_environment_deleted(phase="after delete")
    assert_managed_storage(phase="after delete")

    print("windows installer exact restart/reinstall user path: PASS")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(f"windows installer exact restart/reinstall user path: FAIL: {exc}", file=sys.stderr)
        raise
