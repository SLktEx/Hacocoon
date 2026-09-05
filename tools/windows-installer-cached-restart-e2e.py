#!/usr/bin/env python3
"""Run the exact Windows restart/reinstall E2E through -UseCachedWslImage.

The cache switch is a shipped installer option used by repeated validation. This
wrapper preserves the restart driver's assertions while making both BAT invocations
explicitly exercise the trusted cached WSL image path retained on main.
"""

from __future__ import annotations

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


def main() -> int:
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
    return restart.main()


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:
        print(f"cached Windows installer restart E2E: FAIL: {exc}", file=sys.stderr)
        raise
