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
                "haco-windows.cmd",
                "haco-windows.ps1",
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

            # Keep the current one-shot installer contract from #441.
            required_windows_contract = [
                '[switch]$InteractiveUserSetup',
                '$ManagedLoginUser = "hacocoon"',
                'Ensure-ManagedWslLoginUser',
                'Complete-InteractiveWslUserSetup',
                'Enable-BootstrapSudo',
                'Disable-BootstrapSudo',
                'Get-SudoersPolicyFile',
                'foreach ($policy in @("/etc/sudoers-rs", "/etc/sudoers"))',
                'Set-HacocoonSudoPolicyBlock',
                'Remove-HacocoonSudoPolicyBlock',
                'Set-HacocoonLoginSudoRule',
                '# BEGIN HACOCOON $marker_name',
                '$LoginUser ALL=(ALL:ALL) NOPASSWD: ALL',
                'Validating temporary sudo rule through policy files',
                '"sudo", "-n", "/usr/bin/true"',
                'function Invoke-WslRootShellScript',
                'sh -eu "$tmp" "$@"',
                'Never send installer-controlled bytes through the Windows native stdin',
                'base64 -d >> "$2"',
                '"sh", $encoded, $Path',
                '$mainFailure = $null',
                'Bootstrap sudo cleanup also failed after the installer error',
                '/usr/sbin/visudo -cf "$tmp"',
                'install -o root -g root -m 0440 "$tmp" "$policy"',
                'throw $mainFailure',
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
                '$normalized | & wsl.exe @Arguments',
                '"--exec", "sh", "-s"',
                'function Invoke-WslCaptureWithInput',
                'Get-SudoersPolicyFiles',
                'Write-WslUtf8File $Name $policy $block -Append',
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
                '@include $RulePath',
            ):
                if forbidden_provider_guess in windows_installer:
                    raise SystemExit(
                        f"Windows installer regressed to sudo provider guessing: {forbidden_provider_guess!r}"
                    )

            # Also retain #439's Windows maintenance launcher package contract.
            windows_bat = zf.read("install-windows.bat").decode("utf-8")
            for required in ("haco-windows.ps1", "__install-launcher"):
                if required not in windows_bat:
                    raise SystemExit(
                        f"Windows {arch} installer does not install the host launcher: {required!r}"
                    )

            launcher_cmd = zf.read("haco-windows.cmd").decode("utf-8")
            if "haco-windows.ps1" not in launcher_cmd:
                raise SystemExit(
                    f"Windows {arch} launcher CMD does not delegate to the PowerShell helper"
                )

            launcher = zf.read("haco-windows.ps1").decode("utf-8")
            for required in (
                "Resolve-WslVhdPath",
                "Invoke-Trim",
                '"--terminate", $InstanceName',
                "Wait-WslStopped",
                "Ensure-WslVhdOffline",
                'Invoke-WslExit $Wsl @("--shutdown")',
                "Hacocoon will not stop them",
                "Optimize-VHD",
                "diskpart.exe",
                "compact vdisk",
                "VHD before:",
                "VHD after:",
                "Validating that",
                "maintenance",
                "compact",
            ):
                if required not in launcher:
                    raise SystemExit(
                        f"Windows {arch} maintenance launcher is missing {required!r}"
                    )
            for forbidden in ("--set-sparse", "--allow-unsafe", "sparseVhd"):
                if forbidden in launcher:
                    raise SystemExit(
                        f"Windows {arch} maintenance launcher must not enable experimental sparse VHD mode: {forbidden!r}"
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
