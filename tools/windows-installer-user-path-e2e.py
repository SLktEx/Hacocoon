#!/usr/bin/env python3
"""Drive the Windows installer through the exact commands a real user types.

Before this driver starts, CI may build the PR candidate and prepare ConPTY.
After it starts, product-facing actions are only keyboard input a real user can
enter unchanged. The driver must not add Hacocoon environment variables,
arguments, options, privileged shortcuts, or assertion-only product commands.
"""

from __future__ import annotations

import os
import queue
import re
import sys
import threading
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, Pattern

INSTANCE = "Hacocoon"
PASSWORD = "Hacocoon-E2E-Only-42!"
ENVIRONMENT = "installer-e2e"
PROCESS_TIMEOUT_SECONDS = 900
POST_EXIT_DRAIN_SECONDS = 1.0

# ConPTY needs a terminal process. Start an ordinary cmd.exe and type every
# product-facing Windows command into it exactly as a user does.
TERMINAL_ARGV = ("cmd.exe",)

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
    """Pass the runner environment through unchanged.

    HACO_* in CI is a configuration leak. Fail instead of deleting, adding, or
    rewriting anything before the product sees its environment.
    """

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


def cmd_prompt_count(text: str) -> int:
    return len(CMD_PROMPT_RE.findall(text))


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

        self.argv = list(TERMINAL_ARGV)
        self.proc = PtyProcess.spawn(
            self.argv,
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
        raise RuntimeError(
            f"{phase}: expected normal user-visible output matching {pattern!r}"
        )


def sudo_responders() -> list[Responder]:
    return [
        responder(
            r"\[sudo\]\s+password for [^:]+:\s*$",
            PASSWORD + "\r\n",
            repeat=True,
        ),
        responder(
            r"\[sudo:\s*authenticate\]\s*Password:\s*$",
            PASSWORD + "\r\n",
            repeat=True,
        ),
    ]


def run_bat(package_root: Path, *, phase: str) -> str:
    """Type `install-windows.bat` into a normal Windows command prompt."""

    process = TerminalProcess(cwd=package_root)
    sent_command = False
    sent_exit = False
    prompt_before = 0

    def drive(normalized: str, terminal: TerminalProcess) -> None:
        nonlocal sent_command, sent_exit, prompt_before
        prompts = cmd_prompt_count(normalized)
        if not sent_command and prompts:
            prompt_before = prompts
            terminal.write("install-windows.bat\r\n")
            sent_command = True
            return
        if sent_command and not sent_exit and prompts > prompt_before:
            terminal.write("exit\r\n")
            sent_exit = True

    output = process.run(responders=sudo_responders(), on_output=drive)
    if not sent_command or not sent_exit:
        raise RuntimeError(f"{phase}: install-windows.bat did not return to the user prompt")
    require_output(output, r"Hacocoon", phase=phase)
    return output


def complete_ubuntu_first_launch() -> str:
    """Type the documented `wsl -d Hacocoon` command and complete stock OOBE."""

    process = TerminalProcess()
    user = normal_user_name()
    sent_wsl = False
    sent_linux_exit = False
    sent_cmd_exit = False
    prompt_before = 0

    def drive(normalized: str, terminal: TerminalProcess) -> None:
        nonlocal sent_wsl, sent_linux_exit, sent_cmd_exit, prompt_before
        prompts = cmd_prompt_count(normalized)
        if not sent_wsl and prompts:
            prompt_before = prompts
            terminal.write("wsl -d Hacocoon\r\n")
            sent_wsl = True
            return

        if sent_wsl and not sent_linux_exit:
            linux_prompt = rf"(?m)^{re.escape(user)}@[^:\r\n]+:[^\r\n]*\$\s*$"
            if re.search(linux_prompt, normalized):
                terminal.write("exit\r\n")
                sent_linux_exit = True
                return

        if sent_linux_exit and not sent_cmd_exit and prompts > prompt_before:
            terminal.write("exit\r\n")
            sent_cmd_exit = True

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
        on_output=drive,
    )
    if not sent_wsl or not sent_linux_exit or not sent_cmd_exit:
        raise RuntimeError("Ubuntu first launch did not complete through the exact user path")
    return output


def run_host_session(session: str, *, expected_output: list[str]) -> str:
    """Type the normal WSL entry, then only the ordinary Hacocoon commands."""

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
        raise RuntimeError(
            f"run from extracted Windows package; missing: {', '.join(missing)}"
        )

    print("==> USER TYPES: install-windows.bat")
    first = run_bat(package_root, phase="first install")
    require_output(first, r"wsl -d Hacocoon", phase="first install")

    print("==> USER TYPES: wsl -d Hacocoon")
    complete_ubuntu_first_launch()

    print("==> USER TYPES: install-windows.bat")
    second = run_bat(package_root, phase="install completion")
    require_output(
        second,
        r"Hacocoon WSL installation complete",
        phase="install completion",
    )

    print("==> USER TYPES: wsl -d Hacocoon and normal haco commands")
    run_host_session(
        "before reinstall",
        expected_output=[
            r"(?m)^haco/ubuntu-26\.04\s*$",
            rf"(?m)^name:\s+{re.escape(ENVIRONMENT)}\s*$",
            r"(?m)^Linux\s+",
        ],
    )

    print("==> USER TYPES: install-windows.bat")
    third = run_bat(package_root, phase="reinstall")
    require_output(
        third,
        r"Hacocoon WSL installation complete",
        phase="reinstall",
    )

    print("==> USER TYPES: wsl -d Hacocoon and reuse the existing Environment")
    run_host_session(
        "after reinstall",
        expected_output=[
            rf"(?m)^name:\s+{re.escape(ENVIRONMENT)}\s*$",
            r"(?m)^Linux\s+",
        ],
    )

    print("windows installer exact user path: PASS")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(f"windows installer exact user path: FAIL: {exc}", file=sys.stderr)
        raise
