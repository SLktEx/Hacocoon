#!/usr/bin/env python3
from pathlib import Path
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
launcher_path = ROOT / "scripts" / "install-windows.bat"
installer_path = ROOT / "scripts" / "install-windows.ps1"
linux_installer_path = ROOT / "scripts" / "install.sh"
launcher = launcher_path.read_text(encoding="utf-8").lower()

required = (
    "%~dp0install-windows.ps1",
    "powershell.exe",
    "-noprofile",
    "-executionpolicy bypass",
    "-file",
    "%*",
    "%errorlevel%",
    "exit /b",
)
for needle in required:
    if needle not in launcher:
        raise SystemExit(f"Windows batch launcher is missing required contract: {needle}")

for forbidden in ("set-executionpolicy", "-executionpolicy unrestricted"):
    if forbidden in launcher:
        raise SystemExit(f"Windows batch launcher contains forbidden policy mutation: {forbidden}")

for required_path in (installer_path, linux_installer_path):
    if not required_path.is_file():
        raise SystemExit(f"required Windows installer bundle member is missing: {required_path.name}")

with tempfile.TemporaryDirectory() as temp:
    archive_path = Path(temp) / "hacocoon-windows-installer.zip"
    subprocess.run(
        ["python3", str(ROOT / "tools" / "package_windows_installer.py"), str(archive_path)],
        cwd=ROOT,
        check=True,
    )
    with zipfile.ZipFile(archive_path) as archive:
        names = archive.namelist()
        expected = ["install-windows.bat", "install-windows.ps1", "install.sh"]
        if names != expected:
            raise SystemExit(f"unexpected Windows installer ZIP contents: {names!r}")
        expected_sources = {
            "install-windows.bat": launcher_path,
            "install-windows.ps1": installer_path,
            "install.sh": linux_installer_path,
        }
        for name, source in expected_sources.items():
            if archive.read(name) != source.read_bytes():
                raise SystemExit(f"packaged {name} differs from source")

print("WINDOWS INSTALLER LAUNCHER OK")
