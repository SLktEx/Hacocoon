# Executed by the existing trusted-host setup, after ownership/nesting checks.
# Uses directory/read_file/publish/run from host_tooling.py.
import io
import json
import resource
import zipfile

PACKER_VERSION = "1.16.0"
PACKER_DIGESTS = {
    "amd64": "5edcd14ab59b535040c512dbecd6ec9ef976a000b073c19d93e4c431c948581e",
    "arm64": "cf18f03460d92265d49b56befff333e80641d845822799eab04357c39f75b5d7",
}
NESTED_OWNER = "haco-packer-v1"


def packer_binary():
    arch = architecture()
    directory("/var/cache/hacocoon/host-tooling")
    name = "packer_" + PACKER_VERSION + "_linux_" + arch + ".zip"
    cache = "/var/cache/hacocoon/host-tooling/" + name
    try:
        data = read_file(cache, 128 << 20)
    except FileNotFoundError:
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        with opener.open("https://releases.hashicorp.com/packer/" + PACKER_VERSION + "/" + name, timeout=60) as response:
            if not response.url.startswith("https://"):
                raise ValueError("insecure Packer redirect")
            data = response.read((128 << 20) + 1)
        if len(data) > 128 << 20 or hashlib.sha256(data).hexdigest() != PACKER_DIGESTS[arch]:
            raise ValueError("Packer checksum differs")
        publish(cache, data, 0o644)
    if hashlib.sha256(data).hexdigest() != PACKER_DIGESTS[arch]:
        raise ValueError("Packer cache checksum differs")
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        entries = [entry for entry in archive.infolist() if entry.filename == "packer"]
        if len(entries) != 1 or not 0 < entries[0].file_size <= 128 << 20:
            raise ValueError("invalid Packer executable")
        mode = entries[0].external_attr >> 16
        if mode and stat.S_IFMT(mode) not in (0, stat.S_IFREG):
            raise ValueError("Packer executable is not regular")
        data = archive.read(entries[0])
    publish("/usr/local/bin/packer", data, 0o755)
    run(["/usr/local/bin/packer", "version"])


def nested_query(path):
    limit = (1 << 20) + 1
    def bound_output():
        resource.setrlimit(resource.RLIMIT_FSIZE, (limit, limit))
    with tempfile.TemporaryFile() as output:
        subprocess.run(["/usr/bin/incus", "query", path], env=ENV, stdin=subprocess.DEVNULL,
                       stdout=output, stderr=subprocess.DEVNULL, timeout=30, check=True,
                       preexec_fn=bound_output)
        output.seek(0)
        data = output.read(limit)
    if len(data) >= limit:
        raise ValueError("oversized nested response")
    return json.loads(data)


def nested_incus():
    # Same signed LTS installer as the Physical Host, executed inside haco-host.
    # Patch updates follow normal apt policy; newer series are never downgraded.
    if not os.path.exists("/usr/bin/incus"):
        subprocess.run(["/bin/sh", "-s", "--", "install"], input=INCUS_LTS_SCRIPT.encode(),
                       env=ENV, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                       timeout=480, check=True)
    run(["/usr/bin/systemctl", "enable", "--now", "incus.socket", "incus.service"])
    run(["/usr/bin/incus", "admin", "waitready", "--timeout=60"])
    subprocess.run(["/bin/sh", "-s", "--", "verify-server"], input=INCUS_LTS_SCRIPT.encode(),
                   env=ENV, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                   timeout=60, check=True)
    server = nested_query("/1.0")
    config = server["config"]
    if config.get("images.compression_algorithm", "none") != "none":
        raise ValueError("nested image compression differs")
    if config.get("core.https_address", ""):
        raise ValueError("nested daemon must be local-only")
    owner = config.get("user.hacocoon.packer")
    if owner is None:
        # Never adopt existing workloads or a custom daemon. Incus defaults
        # (empty default project/profile) are the only unowned starting point.
        profiles = nested_query("/1.0/profiles?recursion=1")
        if (config or len(profiles) != 1 or profiles[0]["name"] != "default" or
                profiles[0]["config"] or profiles[0]["devices"] or
                nested_query("/1.0/instances") or nested_query("/1.0/images") or
                nested_query("/1.0/storage-pools") or
                any(n.get("managed") for n in nested_query("/1.0/networks?recursion=1")) or
                nested_query("/1.0/projects") != ["/1.0/projects/default"]):
            raise ValueError("nested Incus already contains foreign resources")
        run(["/usr/bin/incus", "config", "set", "user.hacocoon.packer", NESTED_OWNER])
    elif owner != NESTED_OWNER:
        raise ValueError("nested Incus owner differs")
    run(["/usr/bin/incus", "config", "set", "images.compression_algorithm", "none"])
    pools = nested_query("/1.0/storage-pools?recursion=1")
    pool = next((p for p in pools if p["name"] == "haco-packer"), None)
    if pool is None:
        run(["/usr/bin/incus", "storage", "create", "haco-packer", "dir",
             "user.hacocoon.owner=" + NESTED_OWNER])
    elif pool["driver"] != "dir" or pool["config"].get("user.hacocoon.owner") != NESTED_OWNER:
        raise ValueError("nested storage owner differs")
    networks = nested_query("/1.0/networks?recursion=1")
    network = next((n for n in networks if n["name"] == "haco-packer0"), None)
    if network is None:
        run(["/usr/bin/incus", "network", "create", "haco-packer0",
             "ipv4.address=auto", "ipv4.nat=true", "ipv6.address=none",
             "user.hacocoon.owner=" + NESTED_OWNER])
    elif (network["type"] != "bridge" or not network["managed"] or
          network["config"].get("user.hacocoon.owner") != NESTED_OWNER or
          network["config"].get("ipv4.nat") != "true" or
          network["config"].get("ipv6.address") != "none"):
        raise ValueError("nested network owner differs")
    directory("/var/lib/hacocoon-packer")
    directory("/var/lib/hacocoon-packer/builds")
    os.chmod("/var/lib/hacocoon-packer", 0o700)
    os.chmod("/var/lib/hacocoon-packer/builds", 0o700)


def packer_setup():
    packer_binary()
    nested_incus()
