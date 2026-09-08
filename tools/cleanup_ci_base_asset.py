"""Remove one catalog-owned, unused Base in the disposable storage CLI fixture."""
import json
import os
import pathlib
import re
import stat
import subprocess
import sys

PROJECT = "hacocoon"
POOL = "haco-local-default"


def owned_asset(data, name, require_unused=True):
    if data.get("version") != 7 or (require_unused and (data.get("environments") or data.get("workspace_leases") or data.get("snapshots"))):
        raise RuntimeError("fixture has active ownership or unsupported schema")
    rows = [a for a in data.get("base_assets", {}).values() if a.get("native_ref") == "instance/" + name]
    if len(rows) != 1:
        raise RuntimeError("Base ownership is ambiguous")
    a = rows[0]
    owner = a.get("owner", "")
    revision = a.get("base", {}).get("revision", "")
    if (not re.fullmatch(r"[a-f0-9]{32}", owner) or name != "haco-base-" + owner
            or a.get("id") != "base-" + owner or data["base_assets"].get(a["id"]) != a
            or a.get("provider") != "incus" or a.get("scope") != PROJECT + "/" + POOL
            or a.get("state") != "ready" or not re.fullmatch(r"sha256:[a-f0-9]{64}", revision)):
        raise RuntimeError("Base identity mismatch")
    binding = json.loads(a["binding"])
    if (set(binding) != {"version", "project", "pool", "source"} or binding["version"] != 1
            or binding["project"] != PROJECT or binding["pool"] != POOL
            or binding["source"].split(":", 1)[-1] != revision[7:]):
        raise RuntimeError("Base binding mismatch")
    return a


def validate_instance(a, instance):
    expected = {"user.hacocoon.kind": "base", "user.hacocoon.owner": a["owner"],
                "user.hacocoon.base-name": a["base"]["name"],
                "user.hacocoon.base-revision": a["base"]["revision"],
                "boot.autostart": "false", "security.privileged": "false", "security.nesting": "false"}
    if (instance.get("name") != "haco-base-" + a["owner"] or instance.get("type") != "container"
            or instance.get("status", "").lower() != "stopped" or instance.get("ephemeral") is not False
            or instance.get("profiles") != []):
        raise RuntimeError("Base is not isolated stopped storage")
    for key in ("config", "expanded_config"):
        config = instance.get(key)
        if not isinstance(config, dict) or any(config.get(k) != v for k, v in expected.items()):
            raise RuntimeError("Base markers changed")
        if config.get("volatile.base_image") != a["base"]["revision"][7:]:
            raise RuntimeError("Base image changed")
        if any(v and not k.startswith(("volatile.", "image.")) and expected.get(k) != v for k, v in config.items()):
            raise RuntimeError("unexpected Base configuration")
    root = {"root": {"type": "disk", "path": "/", "pool": POOL}}
    if instance.get("devices") != root or instance.get("expanded_devices") != root:
        raise RuntimeError("unexpected Base devices")


def cleanup(data, name, run):
    asset = owned_asset(data, name)
    def inventory():
        rows = json.loads(run("query", "/1.0/instances?project=" + PROJECT + "&recursion=1"))
        if not isinstance(rows, list) or any(not isinstance(i, dict) or not i.get("name") for i in rows):
            raise RuntimeError("invalid inventory")
        return [i for i in rows if i["name"] == name]
    rows = inventory()
    if not rows:
        return
    if len(rows) != 1:
        raise RuntimeError("duplicate instance")
    validate_instance(asset, rows[0])
    run("delete", name, "--project", PROJECT)
    if inventory():
        raise RuntimeError("Base absence unproven")


def main():
    if os.environ.get("GITHUB_ACTIONS") != "true" or os.environ.get("HACO_CI_RUNNER_ENVIRONMENT") != "github-hosted":
        raise RuntimeError("requires disposable GitHub-hosted fixture")
    root = pathlib.Path(os.environ["RUNNER_TEMP"])
    if not root.is_absolute() or len(sys.argv) != 2:
        raise RuntimeError("invalid fixture arguments")
    path = root / "haco-incus-storage-cli-e2e/state/environments.json"
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd) as file:
        info = os.fstat(file.fileno())
        if not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or info.st_nlink != 1 or info.st_mode & 0o077:
            raise RuntimeError("untrusted fixture catalog")
        data = json.load(file)
    def run(*args):
        result = subprocess.run(["incus", *args], capture_output=True, text=True, timeout=60)
        if result.returncode:
            raise RuntimeError("Incus cleanup operation failed")
        return result.stdout
    cleanup(data, sys.argv[1], run)


if __name__ == "__main__":
    main()
