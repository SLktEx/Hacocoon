#!/usr/bin/env python3
from pathlib import Path
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
ubuntu_installer = ROOT / "scripts" / "install-ubuntu.sh"
main_installer = ROOT / "scripts" / "install.sh"

for path in (ubuntu_installer, main_installer):
    if not path.is_file():
        raise SystemExit(f"required Ubuntu installer bundle member is missing: {path.name}")

with tempfile.TemporaryDirectory() as temp:
    archive_path = Path(temp) / "hacocoon-ubuntu-installer.tar.gz"
    subprocess.run(
        ["python3", str(ROOT / "tools" / "package_ubuntu_installer.py"), str(archive_path)],
        cwd=ROOT,
        check=True,
    )
    with tarfile.open(archive_path, "r:gz") as archive:
        members = archive.getmembers()
        names = [member.name for member in members]
        expected = ["install-ubuntu.sh", "install.sh"]
        if names != expected:
            raise SystemExit(f"unexpected Ubuntu installer archive contents: {names!r}")
        expected_sources = {
            "install-ubuntu.sh": ubuntu_installer,
            "install.sh": main_installer,
        }
        for member in members:
            if not member.isfile() or member.mode != 0o755:
                raise SystemExit(f"unsafe Ubuntu installer member metadata: {member.name}")
            extracted = archive.extractfile(member)
            assert extracted is not None
            if extracted.read() != expected_sources[member.name].read_bytes():
                raise SystemExit(f"packaged {member.name} differs from source")

print("UBUNTU INSTALLER PACKAGE OK")
