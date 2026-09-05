#!/usr/bin/env python3
"""Run Windows restart acceptance through the shipped -UseCachedWslImage path.

The cache switch is a shipped installer option used by repeated validation. This
wrapper preserves the restart driver's assertions while making every BAT
invocation explicitly exercise the trusted cached WSL image path retained on
main. Pull-request CI can select the fast restart gate; main/manual runs keep the
full Environment persistence + reinstall acceptance.
"""

from __future__ import annotations

import argparse
import importlib.util
import sys
from pathlib import Path


def load_module(path: Path, name: str):
    spec = importlib.util.spec_from_file_location(name, path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"cannot load module from {path}")
    module = importlib.util.module_from_spec(spec)
    sys.modules[name] = module
    spec.loader.exec_module(module)
    return module


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--pr-gate",
        action="store_true",
        help=(
            "run the fast cached-install + explicit terminate/restart regression gate; "
            "omit the Environment persistence and reinstall phases reserved for full acceptance"
        ),
    )
    return parser.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv)
    here = Path(__file__).resolve().parent
    restart = load_module(here / "windows-installer-restart-e2e.py", "windows_installer_restart_e2e")
    driver = restart.load_driver()

    original_write = driver.TerminalProcess.write

    def write_with_cached_image(self, text: str) -> None:
        if text == "install-windows.bat\r\n":
            text = "install-windows.bat -UseCachedWslImage\r\n"
        original_write(self, text)

    driver.TerminalProcess.write = write_with_cached_image
    restart.load_driver = lambda: driver
    return restart.main(pr_gate=args.pr_gate)


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(f"cached Windows installer restart E2E: FAIL: {exc}", file=sys.stderr)
        raise
