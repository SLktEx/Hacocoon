"""Synthetic installed-transfer fixtures checked only after confirmed reclamation."""
import json
import re
import subprocess
import tempfile


def validate(record):
    patterns = {"nonce": r"[a-f0-9]{16}", "workspace": r"import-[a-f0-9]{16}",
                "oci": r"oci:import-[a-f0-9]{16}", "snapshot": r"snap-[a-f0-9]{32}",
                "commit": r"[a-f0-9]{40}"}
    if not isinstance(record, dict) or set(record) != {"version", *patterns} or type(record["version"]) is not int or record["version"] != 1:
        raise RuntimeError("Invalid retention fixture manifest")
    for key, pattern in patterns.items():
        if not isinstance(record[key], str) or not re.fullmatch(pattern, record[key]):
            raise RuntimeError("Invalid retention fixture identity")
    return record


def load_manifest(name):
    if not name:
        raise RuntimeError("Required reclamation retention fixture missing")
    with open(name, "rb") as source:
        raw = source.read(4097)
    if len(raw) > 4096:
        raise RuntimeError("Retention manifest oversized")
    return validate(json.loads(raw))


def run(args):
    # No raw guest output is published. Exceptions expose fixed failure categories.
    with tempfile.TemporaryFile() as output:
        try:
            result = subprocess.run(args, stdout=output, stderr=subprocess.DEVNULL, timeout=900)
        except subprocess.TimeoutExpired:
            raise RuntimeError("Retention operation timed out; fixture retained") from None
        if result.returncode:
            raise RuntimeError("Retention operation failed; fixture retained")
        output.seek(0)
        data = output.read(65537)
    if len(data) > 65536:
        raise RuntimeError("Retention response oversized")
    return data.decode("utf-8").strip()


def verify(record, host, guest):
    record = validate(record)
    def saved():
        rows = json.loads(host("snapshot", "list", "--json"))
        if not isinstance(rows, list):
            raise RuntimeError("Snapshot inventory unavailable")
        matches = [row for row in rows if isinstance(row, dict) and row.get("id") == record["snapshot"]]
        if len(matches) != 1 or matches[0].get("state") != "ready" or matches[0].get("oci") is not True or matches[0].get("workspaces", 0) < 1:
            raise RuntimeError("Retained snapshot unavailable or incomplete")
        return matches[0]
    before = saved()
    restored = "reclaim-saved-" + record["nonce"]
    current = "reclaim-current-" + record["nonce"]
    result = json.loads(host("snapshot", "restore", "--json", record["snapshot"], restored))
    if result.get("environment") != restored or result.get("state") != "running" or not result.get("workspace") or not result.get("oci") or result["workspace"] == record["workspace"] or result["oci"] == record["oci"]:
        raise RuntimeError("Snapshot restore independence unproven")
    check = ('set -eu; cd /workspace; test "$(cat .git/refs/heads/main)" = ' + record["commit"] +
             '; test "$(cat committed-locally)" = unpushed; test "$(cat tracked)" = uncommitted; '
             'test "$(cat untracked)" = untracked; test "$(cat continued)" = continued-over-ssh; '
             'test "$(cat /var/lib/hacocoon-oci/transfer-marker)" = oci-kept; ')
    guest(restored, check + 'test "$(cat /root/transfer-marker)" = rootfs-kept')
    host("env", "create", "--workspace", "managed:" + record["workspace"], "--resource", record["oci"], current)
    guest(current, check + 'test ! -e /root/transfer-marker')
    if saved() != before:
        raise RuntimeError("Source snapshot changed during restore")
    # Stop only the new named fixture Envs through ordinary lifecycle. Keep all
    # data/snapshots for inspection; never infer cleanup targets from inventory.
    host("env", "stop", restored)
    host("env", "stop", current)


def verify_installed(record, registration):
    prefix = ["wsl.exe", "--distribution-id", registration, "--user", "root", "--exec", "incus", "exec"]
    def host(*args):
        return run([*prefix, "haco-host", "--project", "hacocoon", "--", "/usr/local/bin/haco", *args])
    def guest(name, script):
        return run([*prefix, "haco-" + name, "--project", "hacocoon", "--", "/bin/sh", "-ec", script])
    verify(record, host, guest)
    print("RECLAIM DETACHED WORKSPACE / OCI / SNAPSHOT RESTORE: PASS", flush=True)
