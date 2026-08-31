#!/usr/bin/env python3
from pathlib import Path
import gzip
import io
import sys
import tarfile

ROOT = Path(__file__).resolve().parents[1]
MEMBERS = (
    (ROOT / "scripts" / "install-ubuntu.sh", "install-ubuntu.sh"),
    (ROOT / "scripts" / "install.sh", "install.sh"),
)


def build(output: Path) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("wb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as gz:
            with tarfile.open(fileobj=gz, mode="w", format=tarfile.PAX_FORMAT) as archive:
                for source, name in MEMBERS:
                    if not source.is_file():
                        raise FileNotFoundError(f"required Ubuntu installer file is missing: {source}")
                    data = source.read_bytes()
                    info = tarfile.TarInfo(name)
                    info.size = len(data)
                    info.mode = 0o755
                    info.uid = 0
                    info.gid = 0
                    info.uname = "root"
                    info.gname = "root"
                    info.mtime = 0
                    archive.addfile(info, io.BytesIO(data))


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("usage: package_ubuntu_installer.py OUTPUT.tar.gz")
    build(Path(sys.argv[1]))
