#!/usr/bin/env python3
from __future__ import annotations

import argparse
import gzip
import hashlib
import io
from pathlib import Path
import shutil
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
ZIP_TIMESTAMP = (1980, 1, 1, 0, 0, 0)
TAR_MTIME = 0
SUPPORTED_ARCHES = ("amd64", "arm64")


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def archive_checksum_line(dist_checksums: Path, archive_name: str) -> str:
    for raw in dist_checksums.read_text(encoding="utf-8").splitlines():
        parts = raw.split(None, 1)
        if len(parts) == 2 and parts[1].lstrip("*") == archive_name:
            return f"{parts[0]}  {archive_name}\n"
    raise ValueError(f"checksum not found for {archive_name}")


def add_zip_file(zf: zipfile.ZipFile, source: Path, name: str, mode: int) -> None:
    if not source.is_file():
        raise FileNotFoundError(f"required installer file is missing: {source}")
    info = zipfile.ZipInfo(name, date_time=ZIP_TIMESTAMP)
    info.compress_type = zipfile.ZIP_DEFLATED
    info.create_system = 3
    info.external_attr = mode << 16
    zf.writestr(info, source.read_bytes())


def add_zip_bytes(zf: zipfile.ZipFile, data: bytes, name: str, mode: int) -> None:
    info = zipfile.ZipInfo(name, date_time=ZIP_TIMESTAMP)
    info.compress_type = zipfile.ZIP_DEFLATED
    info.create_system = 3
    info.external_attr = mode << 16
    zf.writestr(info, data)


def add_tar_bytes(tf: tarfile.TarFile, data: bytes, name: str, mode: int) -> None:
    info = tarfile.TarInfo(name=name)
    info.size = len(data)
    info.mode = mode
    info.mtime = TAR_MTIME
    info.uid = 0
    info.gid = 0
    info.uname = "root"
    info.gname = "root"
    tf.addfile(info, io.BytesIO(data))


def package_windows(output: Path, archive: Path, checksum_line: str, version: str) -> None:
    temporary = output.with_suffix(output.suffix + ".tmp")
    temporary.unlink(missing_ok=True)
    with zipfile.ZipFile(temporary, "w") as zf:
        add_zip_file(zf, ROOT / "scripts" / "install-windows.bat", "install-windows.bat", 0o644)
        add_zip_file(zf, ROOT / "scripts" / "install-windows.ps1", "install-windows.ps1", 0o644)
        add_zip_file(zf, ROOT / "scripts" / "install.sh", "install.sh", 0o755)
        add_zip_file(zf, ROOT / "modules/runtime/incus/packaging/incus-boot-guard.py", "incus-boot-guard.py", 0o755)
        add_zip_file(zf, archive, archive.name, 0o644)
        add_zip_bytes(zf, checksum_line.encode(), "checksums.txt", 0o644)
        add_zip_bytes(zf, f"{version}\n".encode(), "VERSION", 0o644)
    temporary.replace(output)


def package_ubuntu(output: Path, archive: Path, checksum_line: str, version: str) -> None:
    temporary = output.with_suffix(output.suffix + ".tmp")
    temporary.unlink(missing_ok=True)
    with temporary.open("wb") as raw:
        with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as gz:
            with tarfile.open(fileobj=gz, mode="w") as tf:
                add_tar_bytes(tf, (ROOT / "scripts" / "install-ubuntu.sh").read_bytes(), "install-ubuntu.sh", 0o755)
                add_tar_bytes(tf, (ROOT / "scripts" / "install.sh").read_bytes(), "install.sh", 0o755)
                add_tar_bytes(tf, (ROOT / "modules/runtime/incus/packaging/incus-boot-guard.py").read_bytes(), "incus-boot-guard.py", 0o755)
                add_tar_bytes(tf, archive.read_bytes(), archive.name, 0o644)
                add_tar_bytes(tf, checksum_line.encode(), "checksums.txt", 0o644)
                add_tar_bytes(tf, f"{version}\n".encode(), "VERSION", 0o644)
    temporary.replace(output)


def main() -> int:
    parser = argparse.ArgumentParser(description="Build architecture-specific Hacocoon installer bundles")
    parser.add_argument("--dist", type=Path, required=True, help="GoReleaser dist directory")
    parser.add_argument("--output", type=Path, required=True, help="output directory")
    parser.add_argument("--version", required=True, help="release tag embedded in packages")
    parser.add_argument(
        "--arch",
        action="append",
        choices=SUPPORTED_ARCHES,
        dest="arches",
        help="architecture to package; repeat for multiple architectures (default: all)",
    )
    args = parser.parse_args()

    arches = tuple(args.arches) if args.arches else SUPPORTED_ARCHES
    if len(set(arches)) != len(arches):
        raise ValueError("architecture arguments must not be duplicated")

    dist = args.dist.resolve()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    checksums = dist / "checksums.txt"
    if not checksums.is_file():
        raise FileNotFoundError(f"missing GoReleaser checksums: {checksums}")

    release_files: list[Path] = []
    for arch in arches:
        archive = dist / f"haco_linux_{arch}.tar.gz"
        if not archive.is_file():
            raise FileNotFoundError(f"missing release archive: {archive}")
        checksum_line = archive_checksum_line(checksums, archive.name)

        raw_archive = output / archive.name
        shutil.copyfile(archive, raw_archive)
        windows = output / f"hacocoon-windows-{arch}.zip"
        ubuntu = output / f"hacocoon-ubuntu-{arch}.tar.gz"
        package_windows(windows, archive, checksum_line, args.version)
        package_ubuntu(ubuntu, archive, checksum_line, args.version)
        release_files.extend((raw_archive, windows, ubuntu))

    (output / "checksums.txt").write_text(
        "".join(f"{sha256(path)}  {path.name}\n" for path in sorted(release_files)),
        encoding="utf-8",
        newline="\n",
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
