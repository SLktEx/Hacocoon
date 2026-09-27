"""Fixed trusted-Host provisioner, embedded by the Incus adapter; no guest input.

Only selected regular files from the authenticated upstream archive are installed.
No archive paths, links, ownership, services or installation scripts are applied.
"""
import hashlib
import json
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
DOCKER_VERSION = "28.5.2"
DOCKER_DIGESTS = {
    "amd64": "ea90cfd12e1eeb12aa1c971741adb8bd4ed88e2a574eaac13f5029a1dbc6300d",
    "arm64": "9e4f82996ab790724094475ebed33a736434bfe5d45231b676fef22ffb80044d",
}
DOCKER_ARCH = {"amd64": "x86_64", "arm64": "aarch64"}
MAX_ARCHIVE = 512 << 20
BINARIES = ("containerd", "containerd-shim-runc-v2", "ctr", "runc", "nerdctl", "buildkitd", "buildctl")
CNI = ("bridge", "host-local", "loopback", "portmap", "firewall", "tuning")
DOCKER_ENGINE_BINARIES = ("containerd", "containerd-shim-runc-v2", "ctr", "docker-init",
                          "docker-proxy", "dockerd", "runc")
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
DOCKER_UNIT = '''[Unit]
Description=Hacocoon trusted Host Docker Engine
After=network.target
[Service]
Type=notify
ExecStart=/usr/local/lib/hacocoon/docker/dockerd
Environment=PATH=/usr/local/lib/hacocoon/docker:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
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


def download_archive(cache_name, url, digest, redirect_prefix, label):
    cache = "/var/cache/hacocoon/host-tooling"
    directory(cache)
    path = cache + "/" + cache_name
    try:
        data = read_file(path, MAX_ARCHIVE)
        if hashlib.sha256(data).hexdigest() != digest:
            raise ValueError("cached " + label + " digest mismatch")
        return path
    except FileNotFoundError:
        pass
    request = urllib.request.Request(url)
    # No proxy credentials or user configuration are inherited by downloads.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    with opener.open(request, timeout=60) as response:
        if not response.url.startswith(redirect_prefix):
            raise ValueError("insecure " + label + " redirect")
        data = response.read(MAX_ARCHIVE + 1)
    if len(data) > MAX_ARCHIVE or hashlib.sha256(data).hexdigest() != digest:
        raise ValueError(label + " archive digest mismatch")
    publish(path, data, 0o644)
    return path


def download(arch):
    name = "nerdctl-full-" + VERSION + "-linux-" + arch + ".tar.gz"
    url = "https://github.com/containerd/nerdctl/releases/download/v" + VERSION + "/" + name
    return download_archive(name, url, DIGESTS[arch], "https://", "tooling")


def docker_download(arch):
    cache_name = "docker-" + DOCKER_VERSION + "-linux-" + arch + ".tgz"
    name = "docker-" + DOCKER_VERSION + ".tgz"
    url = ("https://download.docker.com/linux/static/stable/" + DOCKER_ARCH[arch] + "/" + name)
    return download_archive(cache_name, url, DOCKER_DIGESTS[arch],
                            "https://download.docker.com/", "Docker")

def select_files(archive, wanted):
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


def selected_files(archive):
    wanted = {"bin/" + name: "/usr/local/bin/" + name for name in BINARIES}
    wanted.update({"libexec/cni/" + name: "/usr/local/libexec/cni/" + name for name in CNI})
    return select_files(archive, wanted)


def selected_docker_files(archive):
    wanted = {"docker/docker": "/usr/local/bin/docker"}
    wanted.update({"docker/" + name: "/usr/local/lib/hacocoon/docker/" + name
                   for name in DOCKER_ENGINE_BINARIES})
    return select_files(archive, wanted)


def publish_archive(path, selector):
    with tarfile.open(path, "r:gz") as archive:
        for target, member in selector(archive):
            with archive.extractfile(member) as stream:
                data = stream.read(member.size + 1)
            if len(data) != member.size:
                raise ValueError("truncated tooling payload")
            publish(target, data, 0o755)


def tooling():
    arch = architecture()
    publish_archive(download(arch), selected_files)
    publish_archive(docker_download(arch), selected_docker_files)
    for name in BINARIES:
        run(["/usr/local/bin/" + name, "--version"])
    run(["/usr/local/bin/docker", "--version"])
    run(["/usr/local/lib/hacocoon/docker/dockerd", "--version"])
    run(["/usr/local/lib/hacocoon/docker/containerd", "--version"])
    run(["/usr/local/lib/hacocoon/docker/runc", "--version"])


def verify_docker_config():
    config = json.loads(read_file("/etc/docker/daemon.json", 8192))
    if not isinstance(config, dict):
        raise ValueError("invalid Docker configuration")
    required = {"data-root": "/var/lib/hacocoon-oci/docker", "exec-root": "/run/docker"}
    if any(config.get(key) != value for key, value in required.items()):
        raise ValueError("conflicting Docker storage configuration")
    if "storage-driver" in config:
        raise ValueError("Docker storage driver is managed by Hacocoon")


def services():
    # This known configuration was installed before the managed source became
    # ready. Updating only that exact form preserves existing data and Docker.
    verify_docker_config()
    publish("/etc/containerd/config.toml", native_config(architecture()).encode(),
            0o644, previous=CONTAINERD.encode())
    publish("/etc/nerdctl/nerdctl.toml", NERDCTL.encode(), 0o644)
    publish("/etc/systemd/system/containerd.service", CONTAINERD_UNIT.encode(), 0o644)
    publish("/etc/systemd/system/docker.service", DOCKER_UNIT.encode(), 0o644)
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
    run(["/usr/bin/systemctl", "enable", "--now", "containerd.service", "buildkit.service",
         "docker.service"])
    for attempt in range(60):
        try:
            run(["/usr/local/bin/ctr", "version"], 5)
            run(["/usr/local/bin/buildctl", "debug", "workers"], 5)
            run(["/usr/local/bin/docker", "info"], 5)
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
