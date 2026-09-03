#!/usr/bin/env python3
"""Exercise the Windows installer with an explicit WSL restart before reinstall.

This orchestration deliberately proves that the first install stands on its own:
after creating a real Environment, the test terminates the Hacocoon WSL distro,
re-enters it, and runs ordinary haco commands before the second BAT is allowed
to run. That prevents the reinstall path from masking restart-persistence bugs.
"""

from __future__ import annotations

import importlib.util
import os
import re
import subprocess
import sys
from pathlib import Path


def load_driver():
    path = Path(__file__).with_name("windows-installer-user-path-e2e.py")
    spec = importlib.util.spec_from_file_location("windows_installer_user_path_e2e", path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load Windows user-path driver from {path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


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


def main() -> int:
    driver = load_driver()

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

    # First install and real Environment creation.
    print("==> ACTION USER TYPES: install-windows.bat")
    first = driver.run_bat(package_root, phase="first install")
    driver.require_output(first, r"Hacocoon WSL installation complete", phase="first install")
    driver.assert_installed_host_state(phase="after first BAT")

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
    driver.assert_storage_state(phase="after Environment create")

    # Critical regression: restart WSL before any second installer invocation.
    # If the first BAT only works because a later BAT repairs runtime state, this
    # phase fails before the reinstall is ever reached.
    run_user_terminate(driver)
    assert_wsl_stopped(driver, phase="after explicit terminate")

    driver.HOST_SESSION_COMMANDS["after terminate"] = (
        f"haco env status {driver.ENVIRONMENT}",
        f"haco env exec {driver.ENVIRONMENT} -- uname -a",
    )
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
    driver.assert_storage_state(phase="after terminate restart")

    # Keep the existing reinstall/idempotency regression, but only after the
    # first-install restart proof has already succeeded.
    driver.wait_for_natural_wsl_stop(phase="before reinstall")

    print("==> ACTION USER TYPES: install-windows.bat")
    second = driver.run_bat(package_root, phase="reinstall")
    driver.require_output(second, r"Hacocoon WSL installation complete", phase="reinstall")
    driver.assert_installed_host_state(phase="after reinstall")
    driver.assert_environment_runtime(present=True, phase="after reinstall")
    driver.assert_storage_state(phase="after reinstall")

    print("==> ACTION USER TYPES: wsl -d Hacocoon and reuse existing Environment")
    driver.run_host_session(
        "after reinstall",
        expected_output=[
            rf"(?m)^name:\s+{re.escape(driver.ENVIRONMENT)}\s*$",
            r"(?m)^Linux\s+",
        ],
    )
    driver.assert_environment_runtime(present=False, phase="after Environment delete")
    driver.assert_storage_state(phase="after Environment delete")

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
