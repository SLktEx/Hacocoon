"""Executed only inside the owned ordinary builder Environment, never on Host."""
import base64
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import socket
import stat
import subprocess
import sys
import time
import urllib.request
import zipfile

VERSION = "1.16.0"
DIGESTS = {
    "x86_64": ("amd64", "5edcd14ab59b535040c512dbecd6ec9ef976a000b073c19d93e4c431c948581e"),
    "aarch64": ("arm64", "cf18f03460d92265d49b56befff333e80641d845822799eab04357c39f75b5d7"),
}
ROOT = Path("/run/hacocoon/packer")
MAX_ARCHIVE = 128 << 20


def download(architecture, digest):
    # urllib uses this Environment's ordinary proxy configuration. No bypass,
    # Host download, injected Policy or reusable credential is provided.
    url = f"https://releases.hashicorp.com/packer/{VERSION}/packer_{VERSION}_linux_{architecture}.zip"
    with urllib.request.urlopen(url, timeout=60) as response:
        data = response.read(MAX_ARCHIVE + 1)
    if len(data) > MAX_ARCHIVE or hashlib.sha256(data).hexdigest() != digest:
        raise ValueError("Packer archive checksum/size differs")
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        members = [entry for entry in archive.infolist() if entry.filename == "packer"]
        if len(members) != 1 or members[0].file_size > MAX_ARCHIVE:
            raise ValueError("Packer executable missing or too large")
        mode = members[0].external_attr >> 16
        if mode and stat.S_IFMT(mode) not in (0, stat.S_IFREG):
            raise ValueError("Packer executable is not a regular file")
        # No archive-selected paths, links or installation scripts are applied.
        with archive.open(members[0]) as source, (ROOT / "bin/packer").open("xb") as target:
            remaining = MAX_ARCHIVE
            while True:
                block = source.read(min(65536, remaining + 1))
                if not block:
                    break
                remaining -= len(block)
                if remaining < 0:
                    raise ValueError("Packer executable too large")
                target.write(block)
    (ROOT / "bin/packer").chmod(0o700)


def prepare():
    os.umask(0o077)
    if os.geteuid() != 0:
        raise ValueError("builder guest root required")
    architecture, digest = DIGESTS[platform.machine()]
    for executable in ("/usr/bin/ssh-keygen", "/usr/sbin/sshd"):
        if not os.access(executable, os.X_OK):
            raise ValueError("Base requires OpenSSH server and client tools")
    # The instance-local tree is never a retained Workspace or Host projection.
    ROOT.mkdir(parents=True, exist_ok=False)
    for directory in ("source", "bin", "home", "cache", "plugins", "tmp"):
        (ROOT / directory).mkdir()
    data = sys.stdin.buffer.read((1 << 20) + 1)
    if len(data) > 1 << 20:
        raise ValueError("build context too large")
    request = json.loads(data)
    files = request["files"]
    if not 1 <= len(files) <= 128:
        raise ValueError("invalid context count")
    total = 0
    for item in files:
        parts = item["path"].split("/")
        if len(item["path"]) > 240 or len(parts) > 8 or any(not re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]*", part) for part in parts):
            raise ValueError("invalid context path")
        content = base64.b64decode(item["data"] or "", validate=True)
        total += len(content)
        if total > 512 << 10:
            raise ValueError("build context too large")
        target = ROOT / "source" / item["path"]
        target.parent.mkdir(parents=True, exist_ok=True)
        with target.open("xb") as output:
            output.write(content)
    download(architecture, digest)
    for name in ("key", "host-key"):
        subprocess.run(["/usr/bin/ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(ROOT / name)], check=True, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    with socket.socket() as reservation:
        reservation.bind(("127.0.0.1", 0))
        port = reservation.getsockname()[1]
    config = f"""Port {port}
ListenAddress 127.0.0.1
HostKey {ROOT}/host-key
PidFile {ROOT}/sshd.pid
AuthorizedKeysFile {ROOT}/key.pub
PermitRootLogin prohibit-password
PasswordAuthentication no
KbdInteractiveAuthentication no
UsePAM yes
AllowAgentForwarding no
AllowTcpForwarding no
X11Forwarding no
PermitTTY no
Subsystem sftp internal-sftp
"""
    for key in ("HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY", "http_proxy", "https_proxy", "no_proxy"):
        value = os.environ.get(key, "")
        if value:
            if not re.fullmatch(r"[A-Za-z0-9.:/,_%\[\]-]+", value):
                raise ValueError("invalid guest proxy setting")
            config += f"SetEnv {key}={value}\n"
    (ROOT / "sshd_config").write_text(config)
    Path("/run/sshd").mkdir(exist_ok=True)
    daemon = subprocess.Popen(["/usr/sbin/sshd", "-D", "-f", str(ROOT / "sshd_config")], stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, start_new_session=True)
    try:
        deadline = time.monotonic() + 5
        while True:
            if daemon.poll() is not None:
                raise ValueError("builder SSH server failed")
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=0.2):
                    break
            except OSError:
                if time.monotonic() >= deadline:
                    raise ValueError("builder SSH server not ready")
                time.sleep(0.05)
        environment = {
            "HACO_PACKER_KEY": str(ROOT / "key"), "HACO_PACKER_PORT": str(port),
            "HOME": str(ROOT / "home"), "TMPDIR": str(ROOT / "tmp"),
            "PACKER_CACHE_DIR": str(ROOT / "cache"), "PACKER_PLUGIN_PATH": str(ROOT / "plugins"),
            "CHECKPOINT_DISABLE": "1", "PACKER_NO_COLOR": "1",
        }
        (ROOT / "environment").write_text("".join(f"{key}='{value}'\n" for key, value in environment.items()))
    except BaseException:
        daemon.terminate()
        daemon.wait(timeout=5)
        raise
    # The whole ordinary Env owns this daemon. Canonical failure cleanup or the
    # successful pre-publication stop terminates it and all provisioning children.


if __name__ == "__main__":
    try:
        prepare()
    except (OSError, ValueError, KeyError, subprocess.SubprocessError):
        print("Packer preparation failed. Check Base Python/OpenSSH tools, normal network approval for releases.hashicorp.com, and the build context.", file=sys.stderr)
        sys.exit(1)
