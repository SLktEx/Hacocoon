#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import importlib.util
from pathlib import Path
import subprocess
import sys
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
            for forbidden in ("haco-windows.cmd", "haco-windows.ps1"):
                if forbidden in names:
                    raise SystemExit(f"Windows {arch} package unexpectedly contains native haco launcher {forbidden}")
            if f"haco_linux_{other}.tar.gz" in names:
                raise SystemExit(f"Windows {arch} package contains the wrong architecture")
            if zf.read("checksums.txt").decode() != checksum_line:
                raise SystemExit(f"Windows {arch} inner checksum mismatch")
            if zf.read("VERSION").decode() != VERSION + "\n":
                raise SystemExit(f"Windows {arch} version mismatch")

            windows_installer = zf.read("install-windows.ps1").decode("utf-8")
            for required in (
                "function Invoke-ElevatedWsl",
                "([Environment]::SystemDirectory)",
                'Join-Path ([Environment]::SystemDirectory) "wsl.exe"',
                "Start-Process -FilePath $systemWsl",
                "-Verb RunAs",
                "Administrator approval is required only to create",
                "Invoke-ElevatedWsl $args",
                "$createExitCode = $LASTEXITCODE",
            ):
                if required not in windows_installer:
                    raise SystemExit(
                        f"Windows {arch} package is missing UAC creation behavior: {required!r}"
                    )
            if "Creating the dedicated Hacocoon WSL instance requires an elevated PowerShell." in windows_installer:
                raise SystemExit(f"Windows {arch} package still contains the old elevation hard failure")
            if '$process = Start-Process -FilePath "wsl.exe"' in windows_installer:
                raise SystemExit(f"Windows {arch} package elevates a PATH-resolved wsl.exe")
            if "$createExitCode = if (Test-Administrator)" in windows_installer:
                raise SystemExit(
                    f"Windows {arch} package captures wsl.exe stdout into its exit-code variable"
                )

            for required in (
                '[switch]$InteractiveUserSetup', '[switch]$UseCachedWslImage',
                '$ManagedLoginUser = "hacocoon"', 'Ensure-ManagedWslLoginUser',
                'Complete-InteractiveWslUserSetup', 'Configure-ManagedWslOobe',
                'Invoke-WslRootShellScript', '"HACO_INSTALL_USER=$loginUser"',
                '--user root --exec env', 'Running common Ubuntu install.sh',
                '$actualSha256 = Get-Sha256Hex $temporaryPath',
                '[Security.Cryptography.SHA256]::Create()',
            ):
                if required not in windows_installer:
                    raise SystemExit(f"Windows installer lost current contract: {required!r}")
            for forbidden in (
                'NOPASSWD', '/etc/sudoers', 'HACO_BOOTSTRAP_LOGIN_USER',
                'Invoke-WslCaptureWithInput', '"--exec", "sh", "-s"',
                'Complete normal Ubuntu user setup, then run this installer again.',
                '"--lock"',
            ):
                if forbidden in windows_installer:
                    raise SystemExit(f"Windows installer restored rejected behavior: {forbidden!r}")
            windows_bat = zf.read("install-windows.bat").decode("utf-8")
            for forbidden in (
                "__install-launcher",
                "WINDOWS_LAUNCHER",
                "HACO_LAUNCHER_EXIT",
                "%LOCALAPPDATA%\\Hacocoon\\bin\\haco.cmd",
                "%LOCALAPPDATA%\\Hacocoon\\bin\\haco-windows.ps1",
            ):
                if forbidden in windows_bat:
                    raise SystemExit(f"Windows {arch} installer still contains native launcher behavior: {forbidden!r}")
            if "Hacocoon Windows installation complete." not in windows_bat:
                raise SystemExit(f"Windows {arch} installer does not expose final completion")

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

# A ConPTY cmd.exe session emits OSC title sequences before and after installer
# output. Normalization must remove each OSC sequence independently instead of
# greedily deleting user-visible text between them.
driver_path = ROOT / "tools" / "windows-installer-user-path-e2e.py"
spec = importlib.util.spec_from_file_location("windows_installer_user_path_e2e_test", driver_path)
if spec is None or spec.loader is None:
    raise SystemExit(f"cannot load Windows user-path driver from {driver_path}")
driver = importlib.util.module_from_spec(spec)
sys.modules[spec.name] = driver
try:
    spec.loader.exec_module(driver)
    osc_sample = (
        "\x1b]0;before\x1b\\"
        "Hacocoon Windows installation complete."
        "\x1b]0;after\x1b\\"
    )
    normalized = driver.normalize_terminal(osc_sample)
finally:
    sys.modules.pop(spec.name, None)
if normalized != "Hacocoon Windows installation complete.":
    raise SystemExit(f"Windows terminal normalization deleted visible output: {normalized!r}")

print("INSTALLER PACKAGES OK")
