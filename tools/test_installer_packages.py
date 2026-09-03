#!/usr/bin/env python3
from __future__ import annotations

import hashlib
from pathlib import Path
import subprocess
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
VERSION = "v9.8.7-test"


def digest(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


with tempfile.TemporaryDirectory() as temp:
    temp_root = Path(temp)
    dist = temp_root / "dist"
    out = temp_root / "out"
    dist.mkdir()

    checksum_lines = []
    for arch in ("amd64", "arm64"):
        archive = dist / f"haco_linux_{arch}.tar.gz"
        archive.write_bytes(f"fake-{arch}-archive\n".encode())
        checksum_lines.append(f"{digest(archive)}  {archive.name}\n")
    (dist / "checksums.txt").write_text("".join(checksum_lines), encoding="utf-8")

    subprocess.run(
        [
            "python3",
            str(ROOT / "tools" / "package_installers.py"),
            "--dist",
            str(dist),
            "--output",
            str(out),
            "--version",
            VERSION,
        ],
        cwd=ROOT,
        check=True,
    )

    expected_release = {
        "haco_linux_amd64.tar.gz",
        "haco_linux_arm64.tar.gz",
        "hacocoon-windows-amd64.zip",
        "hacocoon-windows-arm64.zip",
        "hacocoon-ubuntu-amd64.tar.gz",
        "hacocoon-ubuntu-arm64.tar.gz",
        "checksums.txt",
    }
    actual_release = {path.name for path in out.iterdir()}
    if actual_release != expected_release:
        raise SystemExit(f"unexpected release payload: {sorted(actual_release)!r}")

    for arch in ("amd64", "arm64"):
        other = "arm64" if arch == "amd64" else "amd64"
        archive_name = f"haco_linux_{arch}.tar.gz"
        checksum_line = f"{digest(dist / archive_name)}  {archive_name}\n"

        with zipfile.ZipFile(out / f"hacocoon-windows-{arch}.zip") as zf:
            names = zf.namelist()
            expected = [
                "install-windows.bat",
                "install-windows.ps1",
                "install.sh",
                archive_name,
                "checksums.txt",
                "VERSION",
            ]
            if names != expected:
                raise SystemExit(f"unexpected Windows {arch} package: {names!r}")
            if f"haco_linux_{other}.tar.gz" in names:
                raise SystemExit(f"Windows {arch} package contains the wrong architecture")
            if zf.read("checksums.txt").decode() != checksum_line:
                raise SystemExit(f"Windows {arch} inner checksum mismatch")
            if zf.read("VERSION").decode() != VERSION + "\n":
                raise SystemExit(f"Windows {arch} version mismatch")

            windows_installer = zf.read("install-windows.ps1").decode("utf-8")
            for required in (
                "function Invoke-ElevatedWsl",
                'Start-Process -FilePath "wsl.exe"',
                "-Verb RunAs",
                "Administrator approval is required only to create",
                "Invoke-ElevatedWsl $args",
            ):
                if required not in windows_installer:
                    raise SystemExit(
                        f"Windows {arch} package is missing UAC creation behavior: {required!r}"
                    )
            if "Creating the dedicated Hacocoon WSL instance requires an elevated PowerShell." in windows_installer:
                raise SystemExit(f"Windows {arch} package still contains the old elevation hard failure")

        with tarfile.open(out / f"hacocoon-ubuntu-{arch}.tar.gz", "r:gz") as tf:
            names = tf.getnames()
            expected = ["install-ubuntu.sh", "install.sh", archive_name, "checksums.txt", "VERSION"]
            if names != expected:
                raise SystemExit(f"unexpected Ubuntu {arch} package: {names!r}")
            if f"haco_linux_{other}.tar.gz" in names:
                raise SystemExit(f"Ubuntu {arch} package contains the wrong architecture")
            if tf.extractfile("checksums.txt").read().decode() != checksum_line:
                raise SystemExit(f"Ubuntu {arch} inner checksum mismatch")
            if tf.extractfile("VERSION").read().decode() != VERSION + "\n":
                raise SystemExit(f"Ubuntu {arch} version mismatch")

    release_checksums = {}
    for line in (out / "checksums.txt").read_text(encoding="utf-8").splitlines():
        value, name = line.split(None, 1)
        release_checksums[name] = value
    for name in expected_release - {"checksums.txt"}:
        if release_checksums.get(name) != digest(out / name):
            raise SystemExit(f"release checksum mismatch for {name}")

print("INSTALLER PACKAGES OK")
