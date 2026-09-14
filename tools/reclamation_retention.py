"""Synthetic installed-transfer fixtures checked only after confirmed reclamation."""
import json
import re
import subprocess
import tempfile
from pathlib import Path


def validate(record):
    patterns = {"nonce": r"[a-f0-9]{16}", "workspace": r"import-[a-f0-9]{16}",
                "oci": r"oci:import-[a-f0-9]{16}", "snapshot": r"snap-[a-f0-9]{32}",
                "commit": r"[a-f0-9]{40}", "base": r"win-base-[a-f0-9]{16}",
                "base_revision": r"sha256:[a-f0-9]{64}"}
    if not isinstance(record, dict) or set(record) != {"version", *patterns} or type(record["version"]) is not int or record["version"] != 2:
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


def failure_reason(raw):
    # Return compiled vocabulary only. These observations never authorize a retry.
    for reason in ("not_found", "already_exists", "invalid_argument", "unsupported",
                   "unavailable", "denied", "busy", "incompatible_state",
                   "recovery_required", "canceled", "timeout", "failed"):
        if re.search(rb"(?m)^\[failed\] operation=environment_create stage=controller reason=" +
                     reason.encode() + rb"\r?$", raw):
            return reason
    return "unclassified"


def run(args, timeout=900):
    # No raw guest output is published. Exceptions expose fixed failure categories.
    with tempfile.TemporaryFile() as output, tempfile.TemporaryFile() as diagnostic:
        try:
            result = subprocess.run(args, stdout=output, stderr=diagnostic, timeout=timeout)
        except subprocess.TimeoutExpired:
            raise RuntimeError("Retention operation timed out; fixture retained") from None
        if result.returncode:
            diagnostic.seek(0)
            reason = failure_reason(diagnostic.read(65536))
            raise RuntimeError(f"Retention operation failed: exit_code={result.returncode} reason={reason}; fixture retained")
        output.seek(0)
        data = output.read(65537)
    if len(data) > 65536:
        raise RuntimeError("Retention response oversized")
    return data.decode("utf-8").strip()


def verify(record, host, guest):
    record = validate(record)
    # This journey checks retained data, using the Base already built through
    # the public CLI. It must not resolve a moving external default image again.
    expected_base = {"name": record["base"], "revision": record["base_revision"]}
    base = json.loads(host("base", "inspect", "--json", record["base"]))
    if not isinstance(base, dict) or any(base.get(k) != v for k, v in expected_base.items()):
        raise RuntimeError("Retained Base identity changed or unavailable")
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
    created = json.loads(host("env", "create", "--json", "--base", record["base"],
                              "--workspace", "managed:" + record["workspace"], "--resource", record["oci"], current))
    if not isinstance(created, dict) or created.get("name") != current or created.get("base") != expected_base:
        raise RuntimeError("Reattached Environment Base identity unproven")
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
        try:
            return run([*prefix, "haco-host", "--project", "hacocoon", "--", "/usr/local/bin/haco", *args])
        except RuntimeError:
            # The original operation is never repeated. Only fixed-vocabulary
            # observations are printed; diagnostic failure preserves the primary.
            try:
                script = Path(__file__).with_name("reclamation_diagnostics.py").read_text(encoding="utf-8")
                observed = run([*prefix[:-2], "python3", "-c", script], timeout=30)
                data = json.loads(observed)
                # Re-project even the diagnostic process output as untrusted.
                allowed = {"base_resolution", "project", "root_storage",
                           "routed_substrate", "sandbox_proxy", "instance_init", "runtime_unavailable",
                           "cleanup_incomplete", "stale_identity", "workspace_busy", "storage_busy"}
                labels = data.get("observations", []) if isinstance(data, dict) else []
                if not isinstance(labels, list) or any(not isinstance(x, str) or x not in allowed for x in labels):
                    labels = []
                print(json.dumps({"retention_controller_observations": sorted(set(labels))}), flush=True)
            except (OSError, RuntimeError, ValueError, TypeError):
                print('Retention controller observations unavailable', flush=True)
            raise
    def guest(name, script):
        return run([*prefix, "haco-" + name, "--project", "hacocoon", "--", "/bin/sh", "-ec", script])
    verify(record, host, guest)
    print("RECLAIM DETACHED WORKSPACE / OCI / SNAPSHOT RESTORE: PASS", flush=True)
