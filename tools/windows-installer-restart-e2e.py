#!/usr/bin/env python3
"""Exercise the Windows installer with an explicit WSL restart before reinstall.

The full acceptance path deliberately proves that the first install stands on its
own: after creating a real Environment, the test terminates the Hacocoon WSL
distro, re-enters it, and runs ordinary haco commands before the second BAT is
allowed to run. The reinstall then runs against the still-active distro, proving
ordinary idempotency without depending on WSL idle-shutdown timing.

The PR gate keeps the same exact cached installer and explicit WSL lifecycle
boundary, but stops after proving the installed host survives terminate/restart.
The Environment persistence + reinstall journey remains authoritative on main and
manual acceptance runs.
"""

from __future__ import annotations

import importlib.util
import os
import re
import subprocess
import sys
import time
from contextlib import contextmanager
from pathlib import Path

INSTALL_PROCESS_TIMEOUT_SECONDS = 1800


def load_driver():
    path = Path(__file__).with_name("windows-installer-user-path-e2e.py")
    spec = importlib.util.spec_from_file_location("windows_installer_user_path_e2e", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load Windows user-path driver from {path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


@contextmanager
def timed_phase(name: str):
    started = time.monotonic()
    print(f"==> PHASE START: {name}", flush=True)
    try:
        yield
    except Exception:
        elapsed = time.monotonic() - started
        print(f"==> PHASE FAIL: {name} ({elapsed:.1f}s)", flush=True)
        raise
    else:
        elapsed = time.monotonic() - started
        print(f"==> PHASE END: {name} ({elapsed:.1f}s)", flush=True)


def install_ubuntu_insights_responder(driver) -> None:
    """Model the one-time Ubuntu 26.04 WSL consent prompt as real user input.

    Ubuntu's WSL OOBE asks for Ubuntu Insights consent on the first interactive
    distro entry when no consent exists yet. Choosing `n` is an ordinary user
    action, persists the local per-user consent, and leaves later Hacocoon
    sessions prompt-free without changing product arguments or environment.
    """

    original_run = driver.TerminalProcess.run

    def run_with_insights_consent(
        terminal,
        *,
        responders=None,
        on_output=None,
        timeout=driver.PROCESS_TIMEOUT_SECONDS,
    ):
        combined = list(responders or [])
        combined.append(driver.responder(r"\[Y/n/e\]:\s*$", "n\r\n"))
        return original_run(
            terminal,
            responders=combined,
            on_output=on_output,
            timeout=timeout,
        )

    driver.TerminalProcess.run = run_with_insights_consent


def run_user_terminate(driver) -> None:
    argv = ("wsl.exe", "--terminate", driver.INSTANCE)
    print("==> ACTION USER TYPES:", " ".join(argv))
    completed = subprocess.run(
        list(argv),
        env=driver.inherited_child_environment(),
        timeout=driver.ASSERT_TIMEOUT_SECONDS,
        check=False,
    )
    if completed.returncode != 0:
        raise RuntimeError(
            f"explicit user WSL terminate failed with exit {completed.returncode}: "
            + " ".join(argv)
        )


def assert_wsl_stopped(driver, *, phase: str) -> None:
    running = driver.observe(("wsl.exe", "--list", "--running"), phase=phase)
    if re.search(rf"(?im)\b{re.escape(driver.INSTANCE)}\b", running):
        raise RuntimeError(f"{phase}: {driver.INSTANCE} is still running after wsl --terminate")


def assert_supported_storage_state(driver, *, phase: str) -> None:
    """Assert only the supported public Incus-owned pool contract.

    Incus owns the backing image, loop device and mount path. Those are internal
    implementation details and must not become Windows installer acceptance
    contracts.
    """

    driver.observe_wsl_root(
        "incus", "storage", "show", driver.POOL, "--project", driver.PROJECT, phase=phase
    )

    size = driver.observe_wsl_root(
        "incus", "storage", "get", driver.POOL, "size", "--project", driver.PROJECT, phase=phase
    )
    if size.strip() != "128GiB":
        raise RuntimeError(f"{phase}: pool size is {size!r}, expected 128GiB")

    configured = driver.observe_wsl_root(
        "incus",
        "storage",
        "get",
        driver.POOL,
        "btrfs.mount_options",
        "--project",
        driver.PROJECT,
        phase=phase,
    )
    configured_options = {item for item in configured.strip().split(",") if item}
    required = {"compress=zstd:3", "noatime", "nodiscard"}
    missing = sorted(required - configured_options)
    if missing:
        raise RuntimeError(
            f"{phase}: configured Btrfs options are missing {missing!r}: {configured!r}"
        )
    if "autodefrag" in configured_options:
        raise RuntimeError(f"{phase}: autodefrag must remain disabled: {configured!r}")


def run_pr_gate(driver, package_root: Path) -> None:
    with timed_phase("first cached install"):
        print("==> ACTION USER TYPES: install-windows.bat")
        first = driver.run_bat(package_root, phase="first install")
        driver.require_output(
            first, r"Hacocoon Windows installation complete\.", phase="first install"
        )
        driver.assert_installed_host_state(phase="after first BAT")
        assert_supported_storage_state(driver, phase="after first BAT")

    with timed_phase("explicit WSL terminate"):
        run_user_terminate(driver)
        assert_wsl_stopped(driver, phase="after explicit terminate")

    driver.HOST_SESSION_COMMANDS["pr after terminate"] = ("haco base list",)
    with timed_phase("restart and verify installed host"):
        print("==> ACTION USER TYPES: wsl -d Hacocoon and normal haco command after terminate")
        driver.run_host_session(
            "pr after terminate",
            expected_output=[r"(?m)^haco/ubuntu-26\.04\s*$"],
        )
        driver.assert_installed_host_state(phase="after terminate restart")
        assert_supported_storage_state(driver, phase="after terminate restart")


def run_full_acceptance(driver, package_root: Path) -> None:
    with timed_phase("first cached install"):
        print("==> ACTION USER TYPES: install-windows.bat")
        first = driver.run_bat(package_root, phase="first install")
        driver.require_output(
            first, r"Hacocoon Windows installation complete\.", phase="first install"
        )
        driver.assert_installed_host_state(phase="after first BAT")

    with timed_phase("Environment create and initial assertions"):
        print("==> ACTION USER TYPES: wsl -d Hacocoon and normal haco commands")
        driver.run_host_session(
            "before reinstall",
            expected_output=[
                r"(?m)^haco/ubuntu-26\.04\s*$",
                rf"(?m)^name:\s+{re.escape(driver.ENVIRONMENT)}\s*$",
                r"(?m)^Linux\s+",
            ],
        )
        driver.assert_environment_runtime(present=True, phase="after Environment create")
        assert_supported_storage_state(driver, phase="after Environment create")

    with timed_phase("explicit WSL terminate"):
        run_user_terminate(driver)
        assert_wsl_stopped(driver, phase="after explicit terminate")

    driver.HOST_SESSION_COMMANDS["after terminate"] = (
        f"haco env status {driver.ENVIRONMENT}",
        f"haco env exec {driver.ENVIRONMENT} -- uname -a",
    )
    with timed_phase("restart and reuse existing Environment"):
        print("==> ACTION USER TYPES: wsl -d Hacocoon and reuse Environment after terminate")
        driver.run_host_session(
            "after terminate",
            expected_output=[
                rf"(?m)^name:\s+{re.escape(driver.ENVIRONMENT)}\s*$",
                r"(?m)^Linux\s+",
            ],
        )
        driver.assert_installed_host_state(phase="after terminate restart")
        driver.assert_environment_runtime(present=True, phase="after terminate restart")
        assert_supported_storage_state(driver, phase="after terminate restart")

    with timed_phase("second cached install / idempotency"):
        print("==> ACTION USER TYPES: install-windows.bat")
        second = driver.run_bat(package_root, phase="reinstall")
        driver.require_output(
            second, r"Hacocoon Windows installation complete\.", phase="reinstall"
        )
        driver.assert_installed_host_state(phase="after reinstall")
        driver.assert_environment_runtime(present=True, phase="after reinstall")
        assert_supported_storage_state(driver, phase="after reinstall")

    with timed_phase("Environment reuse and cleanup"):
        print("==> ACTION USER TYPES: wsl -d Hacocoon and reuse existing Environment")
        driver.run_host_session(
            "after reinstall",
            expected_output=[
                rf"(?m)^name:\s+{re.escape(driver.ENVIRONMENT)}\s*$",
                r"(?m)^Linux\s+",
            ],
        )
        driver.assert_environment_runtime(present=False, phase="after Environment delete")
        assert_supported_storage_state(driver, phase="after Environment delete")


def main(*, pr_gate: bool = False) -> int:
    driver = load_driver()
    # Hosted Windows runners can spend well beyond 15 minutes installing apt/
    # Incus while still making progress. Keep the exact user path and give that
    # real install a larger wall-clock ceiling instead of adding product shortcuts.
    driver.PROCESS_TIMEOUT_SECONDS = INSTALL_PROCESS_TIMEOUT_SECONDS
    install_ubuntu_insights_responder(driver)

    if os.name != "nt":
        raise RuntimeError("Windows restart E2E must run on Windows")
    driver.inherited_child_environment()

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

    journey = "PR restart gate" if pr_gate else "full restart/reinstall acceptance"
    print(f"==> WINDOWS INSTALLER JOURNEY: {journey}", flush=True)

    if pr_gate:
        run_pr_gate(driver, package_root)
        print("windows installer cached PR restart gate: PASS")
    else:
        run_full_acceptance(driver, package_root)
        print("windows installer terminate/restart + reinstall user journey: PASS")
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(
            f"windows installer terminate/restart + reinstall user journey: FAIL: {exc}",
            file=sys.stderr,
        )
        raise
