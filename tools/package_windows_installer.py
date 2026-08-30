#!/usr/bin/env python3
from __future__ import annotations

import argparse
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parents[1]
MEMBERS = (
    (ROOT / "scripts" / "install-windows.bat", "install-windows.bat"),
    (ROOT / "scripts" / "install-windows.ps1", "install-windows.ps1"),
)
ZIP_TIMESTAMP = (1980, 1, 1, 0, 0, 0)


def add_member(archive: zipfile.ZipFile, source: Path, name: str) -> None:
    if not source.is_file():
        raise FileNotFoundError(f"required Windows installer file is missing: {source}")

    info = zipfile.ZipInfo(name, date_time=ZIP_TIMESTAMP)
    info.compress_type = zipfile.ZIP_DEFLATED
    info.create_system = 3
    info.external_attr = 0o644 << 16
    archive.writestr(info, source.read_bytes())


def package(output: Path) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    temporary = output.with_name(output.name + ".tmp")
    temporary.unlink(missing_ok=True)

    try:
        with zipfile.ZipFile(temporary, mode="w") as archive:
            for source, name in MEMBERS:
                add_member(archive, source, name)
        temporary.replace(output)
    finally:
        temporary.unlink(missing_ok=True)


def main() -> int:
    parser = argparse.ArgumentParser(description="Build the Hacocoon Windows installer ZIP")
    parser.add_argument("output", type=Path, help="output ZIP path")
    args = parser.parse_args()
    package(args.output.resolve())
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
