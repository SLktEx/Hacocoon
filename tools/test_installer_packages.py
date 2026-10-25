#!/usr/bin/env python3
from __future__ import annotations

import hashlib
import importlib.util
from pathlib import Path
import re
import subprocess
import sys
import tarfile
import tempfile
import warnings
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
        native = dist / f"haco_review_windows_{arch}.zip"
        with zipfile.ZipFile(native, "w") as zf:
            zf.writestr("haco-review.exe", f"fake-review-{arch}".encode())
        checksum_lines.append(f"{digest(native)}  {native.name}\n")
        helper = dist / f"haco_wsl_windows_{arch}.zip"
        with zipfile.ZipFile(helper, "w") as zf:
            zf.writestr("haco-wsl.exe", f"fake-wsl-{arch}".encode())
        checksum_lines.append(f"{digest(helper)}  {helper.name}\n")
        tunnel = dist / f"haco_tunnel_windows_{arch}.zip"
        with zipfile.ZipFile(tunnel, "w") as zf:
            zf.writestr("haco-tunnel.exe", f"fake-tunnel-{arch}".encode())
        checksum_lines.append(f"{digest(tunnel)}  {tunnel.name}\n")
        reclaim = dist / f"haco_reclaim_windows_{arch}.zip"
        with zipfile.ZipFile(reclaim, "w") as zf:
            zf.writestr("haco-reclaim.exe", f"fake-reclaim-{arch}".encode())
        checksum_lines.append(f"{digest(reclaim)}  {reclaim.name}\n")

    (dist / "checksums.txt").write_text("".join(checksum_lines), encoding="utf-8")

    subprocess.run(
        [
            sys.executable,
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
                "windows-review.ps1",
                "haco-review.exe",
                "haco-wsl.exe",
                "haco-tunnel.exe",
                "haco-reclaim.exe",
                "install.sh",
                "incus-lts.sh",
                "setup-wsl-host-interop.py",
                "incus-boot-guard.py",
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
            native_checksum = hashlib.sha256(f"fake-review-{arch}".encode()).hexdigest() + "  haco-review.exe\n"
            native_checksum += hashlib.sha256(f"fake-wsl-{arch}".encode()).hexdigest() + "  haco-wsl.exe\n"
            native_checksum += hashlib.sha256(f"fake-tunnel-{arch}".encode()).hexdigest() + "  haco-tunnel.exe\n"
            native_checksum += hashlib.sha256(f"fake-reclaim-{arch}".encode()).hexdigest() + "  haco-reclaim.exe\n"
            if zf.read("haco-reclaim.exe") != f"fake-reclaim-{arch}".encode():
                raise SystemExit("Wrong Windows reclamation client architecture")
            if zf.read("haco-tunnel.exe") != f"fake-tunnel-{arch}".encode():
                raise SystemExit("Wrong Windows tunnel architecture")
            if zf.read("haco-wsl.exe") != f"fake-wsl-{arch}".encode():
                raise SystemExit("Wrong Windows helper architecture")
            if zf.read("checksums.txt").decode() != checksum_line + native_checksum:
                raise SystemExit(f"Windows {arch} inner checksum mismatch")
            if zf.read("VERSION").decode() != VERSION + "\n":
                raise SystemExit(f"Windows {arch} version mismatch")

            windows_installer = zf.read("install-windows.ps1").decode("utf-8")
            for required in (
                "function Invoke-WslInstall",
                'Join-Path ([Environment]::SystemDirectory) "wsl.exe"',
                "& $systemWsl @Arguments",
                "New-WslInstance $InstanceName $args",
                "Invoke-WslInstall $Name $Arguments",
                "$createExitCode = $LASTEXITCODE",
            ):
                if required not in windows_installer:
                    raise SystemExit(
                        f"Windows {arch} package is missing native WSL creation behavior: {required!r}"
                    )
            if "Creating the dedicated Hacocoon WSL instance requires an elevated PowerShell." in windows_installer:
                raise SystemExit(f"Windows {arch} package still contains the old elevation hard failure")
            if 'Start-Process' in windows_installer or 'Invoke-ElevatedWsl' in windows_installer:
                raise SystemExit(f"Windows {arch} package still creates a separate elevated console")
            if "$createExitCode = if (Test-Administrator)" in windows_installer:
                raise SystemExit(
                    f"Windows {arch} package captures wsl.exe stdout into its exit-code variable"
                )

            for required in (
                '[switch]$InteractiveUserSetup', '[switch]$UseCachedWslImage',
                '[switch]$DownloadWslImageOnly',
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
            # Static source contract only: this does not execute PowerShell,
            # install a Windows component, enter WSL or start reclamation.
            for required in (
                "'haco-wsl.exe' { 'reclamation' }",
                "'haco-tunnel.exe' { 'client' }",
                "'haco-reclaim.exe' { 'reclaim-client' }",
                "$installedReclaim = Install-HacocoonBundledWindowsBinary $PSScriptRoot $managedRegistrationId 'haco-reclaim.exe'",
                "Windows storage reclamation help: & '$quotedReclaim' --help",
                "[IO.File]::Replace($temporary, $target, [System.Management.Automation.Language.NullString]::Value)",
                "$entries.Count -ne 1",
            ):
                if required not in windows_installer:
                    raise SystemExit(f"Windows component placement lost its source contract: {required!r}")
            if re.search(r"(?m)^\s*&\s+\$(?:installedReclaim|quotedReclaim)\b", windows_installer):
                raise SystemExit("Installer must print help without launching the reclamation client")
            if "Windows storage reclamation help: & '$quotedReclaim' --distribution" in windows_installer:
                raise SystemExit("Reclamation help must be a standalone command")
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
            expected = ["install-ubuntu.sh", "install.sh", "incus-lts.sh", "setup-wsl-host-interop.py", "incus-boot-guard.py", archive_name, "checksums.txt", "VERSION"]
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

    # The outer release checksums must not bless a Linux archive that disagrees
    # with the GoReleaser checksum embedded in both installer bundles.
    for arch in ("amd64", "arm64"):
        bad_archive = dist / f"haco_linux_{arch}.tar.gz"
        original = bad_archive.read_bytes()
        bad_archive.write_bytes(original + b"corrupted-after-checksum")
        rejected_out = temp_root / f"rejected-linux-{arch}"
        rejected = subprocess.run(
            [sys.executable, str(ROOT / "tools" / "package_installers.py"),
             "--dist", str(dist), "--output", str(rejected_out),
             "--version", VERSION, "--arch", arch],
            cwd=ROOT, capture_output=True, text=True,
        )
        if rejected.returncode == 0 or "Linux archive checksum mismatch" not in rejected.stderr:
            raise SystemExit(f"corrupt Linux {arch} archive was not refused (exit {rejected.returncode})")
        if list(rejected_out.iterdir()):
            raise SystemExit(f"corrupt Linux {arch} archive published release files")
        bad_archive.write_bytes(original)

    # No Windows component may be published after its verified archive bytes
    # change. Public clients and the internal worker share extraction checks.
    for component, label in (("review", "review"), ("wsl", "WSL helper"),
                             ("tunnel", "tunnel"), ("reclaim", "reclaim client")):
        bad_helper = dist / f"haco_{component}_windows_amd64.zip"
        original = bad_helper.read_bytes()
        bad_helper.write_bytes(original + b"corrupted-after-checksum")
        rejected_out = temp_root / f"rejected-{component}"
        rejected = subprocess.run(
            [sys.executable, str(ROOT / "tools" / "package_installers.py"),
             "--dist", str(dist), "--output", str(rejected_out),
             "--version", VERSION, "--arch", "amd64"],
            cwd=ROOT, capture_output=True, text=True,
        )
        if rejected.returncode == 0 or f"Windows {label} archive checksum mismatch" not in rejected.stderr:
            raise SystemExit(f"corrupt {component} archive was not refused")
        if (rejected_out / "hacocoon-windows-amd64.zip").exists():
            raise SystemExit(f"corrupt {component} published a Windows bundle")
        bad_helper.write_bytes(original)

    def expect_rejected(case: str, expected_error: str, arch: str = "amd64") -> None:
        rejected_out = temp_root / ("rejected-" + case)
        result = subprocess.run(
            [sys.executable, str(ROOT / "tools" / "package_installers.py"),
             "--dist", str(dist), "--output", str(rejected_out),
             "--version", VERSION, "--arch", arch],
            cwd=ROOT, capture_output=True, text=True,
        )
        if result.returncode == 0 or expected_error not in result.stderr:
            raise SystemExit(f"{case} was not refused: {result.stderr!r}")
        if (rejected_out / f"hacocoon-windows-{arch}.zip").exists():
            raise SystemExit(f"{case} published a Windows bundle")
        if (rejected_out / "checksums.txt").exists():
            raise SystemExit(f"{case} published release checksums")

    # Every selected archive requires one unambiguous, valid digest. Missing,
    # duplicated (even identical), or malformed entries cannot bless a bundle.
    checksums_file = dist / "checksums.txt"
    original_checksums = checksums_file.read_text(encoding="utf-8")
    for arch in ("amd64", "arm64"):
        for component in ("linux", "review", "wsl", "tunnel", "reclaim"):
            name = (f"haco_linux_{arch}.tar.gz" if component == "linux"
                    else f"haco_{component}_windows_{arch}.zip")
            line = next(line for line in checksum_lines if line.endswith("  " + name + "\n"))
            variants = {
                "missing": original_checksums.replace(line, ""),
                "duplicate": original_checksums + line,
                "conflicting": original_checksums + "0" * 64 + "  " + name + "\n",
                "invalid": original_checksums.replace(line, "not-a-digest  " + name + "\n"),
            }
            try:
                for case, contents in variants.items():
                    checksums_file.write_text(contents, encoding="utf-8")
                    error = ("invalid checksum for " if case == "invalid"
                             else "expected exactly one checksum for ") + name
                    expect_rejected(f"{component}-{arch}-{case}-checksum", error, arch)
            finally:
                checksums_file.write_text(original_checksums, encoding="utf-8")

    # Preserve standard sha256sum binary-marker input as well as GoReleaser's
    # ordinary two-space form. The emitted installer manifest stays canonical.
    try:
        checksums_file.write_text(original_checksums.replace("  ", " *"), encoding="utf-8")
        marker_out = temp_root / "binary-marker-checksums"
        subprocess.run(
            [sys.executable, str(ROOT / "tools" / "package_installers.py"),
             "--dist", str(dist), "--output", str(marker_out), "--version", VERSION],
            cwd=ROOT, check=True,
        )
        for arch in ("amd64", "arm64"):
            for name in (f"hacocoon-windows-{arch}.zip", f"hacocoon-ubuntu-{arch}.tar.gz"):
                if (marker_out / name).read_bytes() != (out / name).read_bytes():
                    raise SystemExit(f"Binary-marker checksum changed {name}")
    finally:
        checksums_file.write_text(original_checksums, encoding="utf-8")

    # The new public client's archive is mandatory and must contain exactly one
    # nonempty root executable, not a nested or duplicate filename.
    for arch in ("amd64", "arm64"):
        reclaim = dist / f"haco_reclaim_windows_{arch}.zip"
        original = reclaim.read_bytes()
        line = next(line for line in checksum_lines if line.endswith("  " + reclaim.name + "\n"))
        try:
            reclaim.unlink()
            expect_rejected(f"reclaim-{arch}-missing-archive", "FileNotFoundError", arch)
            for case, entries in (
                ("missing", [("other.exe", b"other")]),
                ("nested", [("nested/haco-reclaim.exe", b"nested")]),
                ("empty", [("haco-reclaim.exe", b"")]),
                ("duplicate", [("haco-reclaim.exe", b"first"), ("haco-reclaim.exe", b"second")]),
            ):
                with warnings.catch_warnings():
                    warnings.simplefilter("ignore", UserWarning)
                    with zipfile.ZipFile(reclaim, "w") as zf:
                        for name, data in entries:
                            zf.writestr(name, data)
                checksums_file.write_text(
                    original_checksums.replace(line, f"{digest(reclaim)}  {reclaim.name}\n"),
                    encoding="utf-8",
                )
                expect_rejected(f"reclaim-{arch}-{case}-executable", "Windows reclaim executable missing or invalid", arch)
        finally:
            reclaim.write_bytes(original)
            checksums_file.write_text(original_checksums, encoding="utf-8")

# The release build adds the public Windows companion without changing the
# Linux payload or hiding an architecture in the Windows acceptance projection.
release_config = (ROOT / ".goreleaser.yaml").read_text(encoding="utf-8")
reclaim_build = re.search(r"(?ms)^  - id: haco-reclaim\n(.*?)(?=^  - id:|^archives:)", release_config)
if reclaim_build is None:
    raise SystemExit("Release config does not build the reclamation client")
for required in ("main: ./cmd/haco-reclaim", "binary: haco-reclaim", "env: [CGO_ENABLED=0]",
                 "goos: [windows]", "goarch: [amd64, arm64]", "ldflags: [-s -w]"):
    if required not in reclaim_build.group(1):
        raise SystemExit(f"Reclamation release build lost its contract: {required!r}")
if "-H=windowsgui" in reclaim_build.group(1):
    raise SystemExit("Reclamation client must retain its console")
reclaim_archive = re.search(r"(?ms)^  - id: windows-reclaim\n(.*?)(?=^  - id:|^checksum:)", release_config)
if reclaim_archive is None or any(required not in reclaim_archive.group(1) for required in (
    "ids: [haco-reclaim]", "formats: [zip]", 'name_template: "haco_reclaim_{{ .Os }}_{{ .Arch }}"',
)):
    raise SystemExit("Release config does not archive the reclamation client")
windows_workflow = (ROOT / ".github/workflows/windows-all-scripts-e2e.yml").read_text(encoding="utf-8")
matrix_count = release_config.count("goarch: [amd64, arm64]")
if matrix_count != 10 or f"if ($matrixCount -ne {matrix_count})" not in windows_workflow:
    raise SystemExit("Windows source projection must include all ten release build matrices")
if '"dist/haco_reclaim_windows_amd64.zip"' not in windows_workflow:
    raise SystemExit("Windows source projection must require the reclamation archive")

# A ConPTY cmd.exe session emits OSC title sequences before and after installer
# output. Normalization must remove each OSC sequence independently instead of
# greedily deleting user-visible text between them.
driver_path = ROOT / "test/e2e/windows/install.py"
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

# Cache preparation is an image download, not an incomplete installation whose
# failure can be hidden on cache hits. Keep it independent of WSL lifecycle.
cache_workflow = (ROOT / ".github/workflows/windows-wsl-image-cache.yml").read_text(encoding="utf-8")
if "& $installer -DownloadWslImageOnly" not in cache_workflow:
    raise SystemExit("WSL cache warmer must use the download-only installer mode")
for forbidden in ("-UseCachedWslImage", "-InstanceName", "wsl.exe", "continue-on-error"):
    if forbidden in cache_workflow:
        raise SystemExit(f"WSL cache warmer restored lifecycle work or hidden failure: {forbidden!r}")

print("INSTALLER PACKAGES OK")
