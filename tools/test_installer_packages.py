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

            required_windows_contract = [
                '[switch]$InteractiveUserSetup',
                '$ManagedLoginUser = "hacocoon"',
                'Ensure-ManagedWslLoginUser',
                'Complete-InteractiveWslUserSetup',
                'Enable-BootstrapSudo',
                'Disable-BootstrapSudo',
                'Get-SudoersPolicyFiles',
                '@("/etc/sudoers-rs", "/etc/sudoers")',
                'Validating temporary sudo rule through policy candidates',
                '/etc/sudoers-rs',
                'Ensure-HacocoonSudoRuleLoaded',
                'Remove-HacocoonSudoRuleInclude',
                '@include $RulePath',
                '$LoginUser ALL=NOPASSWD: ALL',
                '"sudo", "-n", "/usr/bin/true"',
                '$OutputEncoding = [Text.UTF8Encoding]::new($false)',
                '& wsl.exe --terminate $Name | Out-Null',
                '& wsl.exe --distribution $Name | Out-Host',
                'Running common Ubuntu install.sh',
            ]
            for contract_marker in required_windows_contract:
                if contract_marker not in windows_installer:
                    raise SystemExit(
                        f"Windows installer lost one-shot bootstrap contract: {contract_marker!r}"
                    )
            forbidden_windows_contract = [
                "Complete normal Ubuntu user setup, then run this installer again.",
                "After completing the Ubuntu user setup, run install-windows.bat again.",
            ]
            for contract_marker in forbidden_windows_contract:
                if contract_marker in windows_installer:
                    raise SystemExit(
                        f"Windows installer regressed to two-invocation setup: {contract_marker!r}"
                    )
            for forbidden_provider_guess in (
                "$provider.Stdout -match '^sudo-rs'",
                '"readlink", "-f", "/usr/bin/sudo"',
                '"update-alternatives"',
            ):
                if forbidden_provider_guess in windows_installer:
                    raise SystemExit(
                        f"Windows installer regressed to sudo provider guessing: {forbidden_provider_guess!r}"
                    )
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
