#!/usr/bin/env python3
"""Run the shared BAT / ordinary-entry / restart / rerun acceptance sequence."""
from __future__ import annotations
import importlib.util
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


def main() -> int:
    load_driver().main()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
