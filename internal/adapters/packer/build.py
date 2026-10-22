"""Trusted haco-host Packer worker; never executed on the Physical Host.

Incus owns images. receipt.json is an operation receipt, not a second catalog.
On failure retain exact project/image identities; never replay or guess cleanup.
"""
import base64
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import shutil
import socket
import stat
import subprocess
import sys
import tempfile

ROOT = Path("/var/lib/hacocoon-packer/builds")
OWNER = "user.hacocoon.packer-build"
ENV = {"PATH": "/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
       "HOME": "/root", "LANG": "C.UTF-8", "CHECKPOINT_DISABLE": "1",
       "PACKER_NO_COLOR": "1", "INCUS_SOCKET": "/var/lib/incus/unix.socket"}


def trusted_host():
    # The outer Incus supplies this API. A client environment flag is not proof.
    connection = http.client.HTTPConnection("localhost", timeout=5)
    connection.sock = socket.socket(socket.AF_UNIX)
    connection.sock.settimeout(5)
    connection.sock.connect("/dev/incus/sock")
    try:
        connection.request("GET", "/1.0/config/user.hacocoon.role")
        response = connection.getresponse()
        if response.status != 200 or response.read(128).strip() != b"trusted-host":
            raise ValueError("Packer requires trusted haco-host")
    finally:
        connection.close()


def save(directory, value):
    temporary = directory / "receipt.next"
    with temporary.open("x", encoding="utf-8") as stream:
        json.dump(value, stream, sort_keys=True)
        stream.flush()
        os.fsync(stream.fileno())
    os.replace(temporary, directory / "receipt.json")
    fd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


class Build:
    def __init__(self, identity, base_name, max_bytes):
        if not 0 < max_bytes <= (1 << 63) - 2:
            raise ValueError("invalid artifact limit")
        self.max_bytes = max_bytes
        if not re.fullmatch("[a-f0-9]{32}", identity):
            raise ValueError("invalid build identity")
        if not re.fullmatch(r"[a-z0-9][a-z0-9.-]{0,62}", base_name):
            raise ValueError("invalid Base name")
        self.directory = ROOT / identity
        self.value = {"id": identity, "base_name": base_name, "state": "preparing", "stage": "prepare",
                      "project": "haco-packer-" + identity, "max_bytes": max_bytes}
        self.env = dict(ENV, INCUS_CONF=str(self.directory / "incus"),
                        PACKER_CACHE_DIR=str(self.directory / "cache"),
                        PACKER_PLUGIN_PATH=str(self.directory / "plugins"),
                        TMPDIR=str(self.directory / "tmp"), HACO_PACKER_BUILD_ID=identity)

    def record(self, **changes):
        self.value.update(changes)
        save(self.directory, self.value)

    def run(self, *args, capture=False, timeout=120):
        # Diagnostic reads are bounded in memory; explicit image limits also
        # bound file growth. Never include subprocess text in errors or logs.
        with tempfile.TemporaryFile(dir=self.directory) as output, tempfile.TemporaryFile(dir=self.directory) as error:
            try:
                subprocess.run(args, env=self.env, cwd=self.directory / "source",
                               stdin=subprocess.DEVNULL, stdout=output, stderr=error,
                               timeout=timeout, check=True)
            except (subprocess.CalledProcessError, subprocess.TimeoutExpired) as failure:
                output.seek(0)
                error.seek(0)
                stdout, stderr = output.read(16385), error.read(16385)
                self.record(execution={"exit_code": getattr(failure, "returncode", -1),
                                       "stdout": stdout[:16384].decode("utf-8", errors="replace"),
                                       "stderr": stderr[:16384].decode("utf-8", errors="replace"),
                                       "stdout_truncated": len(stdout) > 16384,
                                       "stderr_truncated": len(stderr) > 16384})
                raise
            if not capture:
                return None
            output.seek(0)
            data = output.read((4 << 20) + 1)
            if len(data) > 4 << 20:
                raise ValueError("oversized provider result")
            return data

    def query(self, path):
        if path.startswith(("/1.0/images", "/1.0/instances")):
            path += ("&" if "?" in path else "?") + "project=" + self.value["project"]
        return json.loads(self.run("incus", "query", path, capture=True))

    def project(self):
        result = self.query("/1.0/projects/" + self.value["project"])
        if result["name"] != self.value["project"] or result["config"].get(OWNER) != self.value["id"]:
            raise ValueError("project ownership changed")

    def prepare(self, template):
        trusted_host()
        self.directory.mkdir(mode=0o700)
        parent = os.open(ROOT, os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(parent)
        finally:
            os.close(parent)
        for name in ("source", "incus", "cache", "plugins", "tmp", "export"):
            (self.directory / name).mkdir(mode=0o700)
        self.record()
        files = template["files"]
        if not 1 <= len(files) <= 128:
            raise ValueError("invalid context")
        total = 0
        for item in files:
            parts = item["path"].split("/")
            if len(item["path"]) > 240 or len(parts) > 8 or any(not re.fullmatch(r"[A-Za-z0-9_][A-Za-z0-9_.-]*", p) for p in parts):
                raise ValueError("invalid context path")
            data = base64.b64decode(item["data"] or "", validate=True)
            total += len(data)
            if total > 512 << 10:
                raise ValueError("oversized context")
            target = self.directory / "source" / item["path"]
            target.parent.mkdir(parents=True, exist_ok=True)
            with target.open("xb") as stream:
                stream.write(data)
        for stage in ("fmt", "init", "validate"):
            self.record(stage=stage)
            self.run("/usr/local/bin/packer", stage, ".", timeout=None)
        self.record(stage="project", state="recovery-required")
        self.run("incus", "project", "create", self.value["project"], "-c", OWNER + "=" + self.value["id"],
                 "-c", "features.images=true", "-c", "features.profiles=true", "-c", "features.networks=false")
        self.project()
        self.run("incus", "project", "switch", self.value["project"])
        self.run("incus", "profile", "set", "default", "security.idmap.size", "65536")
        self.run("incus", "profile", "device", "add", "default", "root", "disk", "path=/", "pool=haco-packer")
        self.run("incus", "profile", "device", "add", "default", "eth0", "nic", "network=haco-packer0", "name=eth0")
        self.record(stage="build")
        self.run("/usr/local/bin/packer", "build", "-color=false", "-on-error=abort", ".", timeout=None)
        self.project()
        images = self.query("/1.0/images?recursion=1")
        built = [image for image in images if image.get("properties", {}).get(OWNER) == self.value["id"]]
        if len(built) != 1:
            raise ValueError("exactly one owned Packer output image required")
        image = built[0]
        fingerprint = image["fingerprint"]
        if not re.fullmatch("[a-f0-9]{64}", fingerprint) or image["type"] != "container" or image["public"]:
            raise ValueError("invalid output image")
        architecture = {"x86_64": "x86_64", "aarch64": "aarch64", "amd64": "x86_64", "arm64": "aarch64"}[image["architecture"]]
        self.record(stage="export", fingerprint=fingerprint, architecture=architecture)
        self.run("incus", "image", "export", fingerprint, str(self.directory / "export" / "image"), timeout=None)
        exported = list((self.directory / "export").iterdir())
        if len(exported) != 1 or exported[0].name != "image.tar":
            raise ValueError("expected unified uncompressed native container archive")
        digest = hashlib.sha256()
        fd = os.open(exported[0], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
        with os.fdopen(fd, "rb") as stream:
            info = os.fstat(stream.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or not 0 < info.st_size <= self.max_bytes:
                raise ValueError("invalid export file")
            count = 0
            for chunk in iter(lambda: stream.read(1 << 20), b""):
                count += len(chunk)
                if count > self.max_bytes:
                    raise ValueError("oversized export")
                digest.update(chunk)
            if count != info.st_size:
                raise ValueError("export changed")
            os.fsync(stream.fileno())
        self.record(stage="cleanup", size=info.st_size, sha256=digest.hexdigest())
        self.cleanup_nested()
        self.record(stage="import", state="artifact-ready")

    def cleanup_nested(self):
        self.project()
        # Upstream logs instance-cleanup errors without necessarily failing build.
        if self.query("/1.0/instances?recursion=1") != []:
            raise ValueError("Packer left build instances; retain exact project")
        images = self.query("/1.0/images?recursion=1")
        identities = [image["fingerprint"] for image in images]
        if any(not re.fullmatch("[a-f0-9]{64}", f) for f in identities) or len(identities) != len(set(identities)):
            raise ValueError("invalid cleanup identities")
        self.record(cleanup_images=identities)
        # Even downloaded source images belong to this fresh private project.
        for fingerprint in identities:
            self.project()
            self.run("incus", "image", "delete", fingerprint)
        if self.query("/1.0/images?recursion=1") != []:
            raise ValueError("image cleanup unconfirmed")
        # Incus deletes the private default profile with its project.
        self.run("incus", "project", "switch", "default")
        self.project()
        self.run("incus", "project", "delete", self.value["project"])
        projects = self.query("/1.0/projects?recursion=1")
        if any(p["name"] == self.value["project"] for p in projects):
            raise ValueError("project cleanup unconfirmed")
        for name in ("source", "incus", "cache", "plugins", "tmp"):
            shutil.rmtree(self.directory / name)


if __name__ == "__main__":
    build = None
    try:
        os.umask(0o077)
        build = Build(sys.argv[1], sys.argv[2], int(sys.argv[3]))
        raw = sys.stdin.buffer.read((1 << 20) + 1)
        if len(raw) > 1 << 20:
            raise ValueError("oversized context")
        build.prepare(json.loads(raw))
    except BaseException:
        if build is not None and (build.directory / "receipt.json").exists():
            build.record(state="recovery-required" if build.value["state"] == "recovery-required" else "failed")
        sys.exit(1)
