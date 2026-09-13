"""Fixed trusted-Host provisioner, embedded by the Incus adapter; no guest input.

Only selected regular files from the authenticated upstream archive are installed.
No archive paths, links, ownership, services or installation scripts are applied.
"""
import hashlib
import os
import platform
import stat
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request

VERSION = "2.3.5"
DIGESTS = {
    "amd64": "b697295c623639734aaab737523c808fd3cc8d3046039fd94fff1744e4c317aa",
    "arm64": "6e4b687f1d138e750a3c8372abc0f81d3d7490b6359c48c0562fc7dfe98859b2",
}
MAX_ARCHIVE = 512 << 20
BINARIES = ("containerd", "containerd-shim-runc-v2", "ctr", "runc", "nerdctl", "buildkitd", "buildctl")
CNI = ("bridge", "host-local", "loopback", "portmap", "firewall", "tuning")
ENV = {"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
       "HOME": "/root", "LANG": "C.UTF-8", "DEBIAN_FRONTEND": "noninteractive"}
CONTAINERD = '''version = 2
root = "/var/lib/hacocoon-oci/containerd"
state = "/run/containerd"
[grpc]
  address = "/run/containerd/containerd.sock"
'''
NERDCTL = '''address = "/run/containerd/containerd.sock"
namespace = "default"
snapshotter = "native"
cni_path = "/usr/local/libexec/cni"
'''
CONTAINERD_UNIT = '''[Unit]
Description=Hacocoon trusted Host containerd
After=network.target
[Service]
Type=notify
ExecStart=/usr/local/bin/containerd --config /etc/containerd/config.toml
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
Delegate=yes
KillMode=process
Restart=on-failure
[Install]
WantedBy=multi-user.target
'''


def architecture():
    return {"x86_64": "amd64", "aarch64": "arm64"}[platform.machine()]


def native_config(arch):
    return (CONTAINERD + '\n[[plugins."io.containerd.transfer.v1.local".unpack_config]]\n'
            + '  platform = "linux/' + arch + '"\n  snapshotter = "native"\n')


def directory(path):
    """Reject links and writable/foreign ancestors before using fixed paths."""
    if path == "/":
        return
    directory(os.path.dirname(path))
    try:
        os.mkdir(path, 0o755)
    except FileExistsError:
        pass
    info = os.lstat(path)
    if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
        raise ValueError("unsafe tooling directory")


def read_file(path, limit):
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as stream:
        info = os.fstat(stream.fileno())
        if (not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or
                info.st_nlink != 1 or info.st_mode & 0o022 or info.st_size > limit):
            raise ValueError("unsafe tooling file")
        data = stream.read(limit + 1)
        if len(data) > limit:
            raise ValueError("oversized tooling file")
        return data


def publish(path, data, mode, previous=None):
    """Publish each fixed file atomically; never overwrite a custom installation."""
    directory(os.path.dirname(path))
    try:
        current = read_file(path, max(len(data), len(previous or b"")))
    except FileNotFoundError:
        current = None
    if current == data:
        if stat.S_IMODE(os.lstat(path).st_mode) != mode:
            raise ValueError("unexpected tooling mode")
        return False
    if current is not None and current != previous:
        raise ValueError("conflicting tooling configuration")
    fd, temporary = tempfile.mkstemp(prefix=".haco-tool-", dir=os.path.dirname(path))
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
            os.fchmod(stream.fileno(), mode)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, path)
        parent = os.open(os.path.dirname(path), os.O_RDONLY | os.O_DIRECTORY)
        try:
            os.fsync(parent)
        finally:
            os.close(parent)
    finally:
        if os.path.exists(temporary):
            os.unlink(temporary)
    return True


def run(argv, timeout=120):
    return subprocess.run(argv, env=ENV, stdin=subprocess.DEVNULL,
                          stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                          check=True, timeout=timeout)


def packages():
    # Ubuntu's signed configured repositories, including universe. Do not add
    # third-party apt keys/sources or install/upgrade unrelated runtime packages.
    required = ("git", "gh", "ca-certificates", "iptables")
    missing = []
    for name in required:
        result = subprocess.run(["/usr/bin/dpkg-query", "-W", "-f=${Status}", name],
                                env=ENV, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                timeout=15, check=False)
        if result.returncode or result.stdout != b"install ok installed":
            missing.append(name)
    if missing:
        run(["/usr/bin/apt-get", "-o", "Acquire::Retries=2", "update"], 180)
        run(["/usr/bin/apt-get", "-o", "DPkg::Lock::Timeout=60", "install", "-y",
             "--no-install-recommends", *missing], 360)
    run(["/usr/bin/git", "--version"])
    run(["/usr/bin/gh", "--version"])


def download(arch):
    cache = "/var/cache/hacocoon/host-tooling"
    directory(cache)
    name = "nerdctl-full-" + VERSION + "-linux-" + arch + ".tar.gz"
    path = cache + "/" + name
    try:
        data = read_file(path, MAX_ARCHIVE)
        if hashlib.sha256(data).hexdigest() != DIGESTS[arch]:
            raise ValueError("cached tooling digest mismatch")
        return path
    except FileNotFoundError:
        pass
    request = urllib.request.Request("https://github.com/containerd/nerdctl/releases/download/v"
                                     + VERSION + "/" + name)
    # No proxy credentials or user configuration are inherited by downloads.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=60) as response:
        if not response.url.startswith("https://"):
            raise ValueError("insecure tooling redirect")
        data = response.read(MAX_ARCHIVE + 1)
    if len(data) > MAX_ARCHIVE or hashlib.sha256(data).hexdigest() != DIGESTS[arch]:
        raise ValueError("tooling archive digest mismatch")
    publish(path, data, 0o644)
    return path


def selected_files(archive):
    wanted = {"bin/" + name: "/usr/local/bin/" + name for name in BINARIES}
    wanted.update({"libexec/cni/" + name: "/usr/local/libexec/cni/" + name for name in CNI})
    selected = {}
    total = 0
    # Never extractall: absolute paths, links, special files and unknown archive
    # entries cannot choose destinations. Validate every selected entry first.
    for member in archive:
        if member.name not in wanted:
            continue
        if (member.name in selected or not member.isfile() or member.size <= 0
                or member.size > 128 << 20 or member.mode & 0o6000):
            raise ValueError("invalid tooling archive entry")
        total += member.size
        if total > MAX_ARCHIVE:
            raise ValueError("oversized tooling payload")
        selected[member.name] = member
    if set(selected) != set(wanted):
        raise ValueError("incomplete tooling archive")
    return [(wanted[name], selected[name]) for name in wanted]


def tooling():
    path = download(architecture())
    with tarfile.open(path, "r:gz") as archive:
        for target, member in selected_files(archive):
            with archive.extractfile(member) as stream:
                data = stream.read(member.size + 1)
            if len(data) != member.size:
                raise ValueError("truncated tooling payload")
            publish(target, data, 0o755)
    for name in BINARIES:
        run(["/usr/local/bin/" + name, "--version"])


def services():
    # This known configuration was installed before the managed source became
    # ready. Updating only that exact form preserves existing data and Docker.
    publish("/etc/containerd/config.toml", native_config(architecture()).encode(),
            0o644, previous=CONTAINERD.encode())
    publish("/etc/nerdctl/nerdctl.toml", NERDCTL.encode(), 0o644)
    publish("/etc/systemd/system/containerd.service", CONTAINERD_UNIT.encode(), 0o644)
    publish("/etc/systemd/system/buildkit.service.d/10-hacocoon-readiness.conf",
            b"[Service]\nType=notify\n", 0o644)
    # persistentOCIConfiguration already owns buildkit.service and its cache
    # root. Refuse drift; never start an arbitrary replacement unit as success.
    expected = '''[Unit]
Description=Environment-local BuildKit with persistent OCI cache
After=network.target
[Service]
ExecStart=/usr/local/bin/buildkitd --root /var/lib/hacocoon-oci/buildkit --addr unix:///run/buildkit/buildkitd.sock --oci-worker-snapshotter native --containerd-worker=false
[Install]
WantedBy=multi-user.target
'''
    if read_file("/etc/systemd/system/buildkit.service", 8192) != expected.encode():
        raise ValueError("conflicting BuildKit configuration")
    run(["/usr/bin/systemctl", "daemon-reload"])
    # enable --now is idempotent and does not interrupt existing containers or
    # builds on repeat setup. Fresh publication precedes first service start.
    run(["/usr/bin/systemctl", "enable", "--now", "containerd.service", "buildkit.service"])
    for attempt in range(60):
        try:
            run(["/usr/local/bin/ctr", "version"], 5)
            run(["/usr/local/bin/buildctl", "debug", "workers"], 5)
            return
        except (subprocess.CalledProcessError, subprocess.TimeoutExpired):
            time.sleep(1)
    raise ValueError("OCI services did not become ready")


if __name__ == "__main__":
    try:
        {"host_packages": packages, "host_tooling": tooling, "host_services": services}[sys.argv[1]]()
    except Exception:
        # Fixed failure only: package/download/helper errors can contain secrets.
        raise SystemExit(1) from None
