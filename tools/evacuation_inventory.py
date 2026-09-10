#!/usr/bin/env python3
"""Read-only Incus inventory for manual evacuation planning, not a backup."""
import json
import hashlib
import os
import stat
import re
import subprocess
import sys
import time
from urllib.parse import quote, urlencode

LIMIT = 4096


class InventoryLimit(Exception):
    pass


def query(url):
    result = subprocess.run(["incus", "query", url], capture_output=True,
                            timeout=30, check=False)
    if result.returncode or len(result.stdout) > 8 * 1024 * 1024:
        raise ValueError("query unavailable or oversized")
    return json.loads(result.stdout)


def text(value):
    if not isinstance(value, str) or len(value) > 4096:
        raise ValueError("invalid metadata")
    return value


def rows(value):
    if not isinstance(value, list) or len(value) > LIMIT:
        raise ValueError("invalid inventory")
    seen = set()
    for item in value:
        if not isinstance(item, dict):
            raise ValueError("invalid row")
        name = text(item.get("name"))
        key = (item.get("type"), name)
        if key in seen:
            raise ValueError("duplicate row")
        seen.add(key)
    return value


def source_reference(source):
    """Classify a native reference without opening it or publishing URI secrets."""
    source = text(source)
    result = {"source_review": "required"}
    if not source:
        result["source_kind"] = "unspecified"
    elif source.startswith("/"):
        result.update(source_kind="host-path-reference", source=source)
    elif re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]*", source):
        result.update(source_kind="volume-or-backend-reference", source=source)
    else:
        result["source_kind"] = "unreported-reference-review-in-incus"
    return result


def disk_bindings(instance):
    """Describe native attachments without opening sources or trusting ownership."""
    devices = instance.get("expanded_devices", instance.get("devices", {}))
    if not isinstance(devices, dict) or len(devices) > LIMIT:
        raise ValueError("invalid devices")
    result = []
    for name, device in devices.items():
        if not isinstance(device, dict):
            raise ValueError("invalid device")
        if device.get("type") != "disk":
            continue
        binding = {"device": text(name), "pool": text(device.get("pool", "")),
                   "path": text(device.get("path", "")), "source_review": "required"}
        binding.update(source_reference(device.get("source", "")))
        if binding["source_kind"] == "unspecified":
            binding["source_kind"] = "instance-root-or-unspecified"
        result.append(binding)
    return result

def inventory(fetch=query):
    report = {"format": 1, "backup_complete": False, "native_queries_complete": False,
              "projects": [], "pools": [], "errors": [], "unreviewed": [
                  "catalog-to-native ownership and data associations",
                  "controller settings and Policy; trusted Host credentials",
                  "manual files and unregistered persistent data inside WSL",
                  "Base provenance, build definitions and saved-only data associations",
                  "external pools/VHDs and Windows drive references",
                  "readability, consistent capture, external destination and restore comparison"]}

    requests = 0
    deadline = time.monotonic() + 300

    def read(url, label):
        nonlocal requests
        if requests >= 256 or time.monotonic() >= deadline:
            raise InventoryLimit()
        requests += 1
        try:
            return rows(fetch(url))
        except (ValueError, TypeError, KeyError, OSError, subprocess.SubprocessError):
            report["errors"].append(label)
            return []

    try:
        pools = read("/1.0/storage-pools?recursion=1", "pools")
        for pool in pools:
            record = {"name": pool["name"], "driver": text(pool.get("driver", ""))}
            try:
                config = pool.get("config", {})
                if not isinstance(config, dict):
                    raise ValueError("invalid pool configuration")
                record.update(source_reference(config.get("source", "")))
            except (ValueError, TypeError):
                record.update(source_review="required", source_kind="unavailable")
                report["errors"].append("pool-source:" + pool["name"])
            report["pools"].append(record)
        for project in read("/1.0/projects?recursion=1", "projects"):
            name = project["name"]
            suffix = urlencode({"project": name, "recursion": 1})
            entry = {"name": name, "instances": [], "volumes": []}
            report["projects"].append(entry)
            for instance in read("/1.0/instances?" + suffix, "instances:" + name):
                ident = quote(instance["name"], safe="")
                snapshots = read("/1.0/instances/" + ident + "/snapshots?" + suffix,
                                 "instance-snapshots:" + name + "/" + instance["name"])
                try:
                    disks = disk_bindings(instance)
                except (ValueError, TypeError):
                    disks = None
                    report["errors"].append("disks:" + name + "/" + instance["name"])
                entry["instances"].append({"name": instance["name"],
                                          "type": text(instance.get("type", "")),
                                          "status": text(instance.get("status", "")),
                                          "disks": disks,
                                          "snapshots": [x["name"] for x in snapshots]})
            for pool in pools:
                base = "/1.0/storage-pools/" + quote(pool["name"], safe="") + "/volumes"
                for volume in read(base + "?" + suffix, "volumes:" + name + "/" + pool["name"]):
                    kind = text(volume.get("type", ""))
                    record = {"name": volume["name"], "type": kind, "pool": pool["name"]}
                    try:
                        record["content_type"] = text(volume.get("content_type", ""))
                    except ValueError:
                        record["content_type"] = None
                        report["errors"].append("volume-content:" + name + "/" + pool["name"] + "/" + volume["name"])
                    # Instance snapshots were listed above; image cache is not a saved snapshot.
                    if kind == "custom":
                        endpoint = base + "/custom/" + quote(volume["name"], safe="")
                        saved = read(endpoint + "/snapshots?" + suffix,
                                     "volume-snapshots:" + name + "/" + pool["name"] + "/" + volume["name"])
                        record["snapshots"] = [x["name"] for x in saved]
                    entry["volumes"].append(record)
    except InventoryLimit:
        report["errors"].append("native-query-budget-exhausted")
    report["native_queries_complete"] = not report["errors"]
    return report


def catalog_inventory(path):
    """Observe one catalog file without migration, locking writes or authority."""
    if not hasattr(os, "O_NOFOLLOW"):
        raise OSError("catalog observation requires Linux")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as source:
        before = os.fstat(source.fileno())
        if not stat.S_ISREG(before.st_mode) or before.st_size > 16 * 1024 * 1024:
            raise ValueError("invalid catalog file")
        raw = source.read(16 * 1024 * 1024 + 1)
        after = os.fstat(source.fileno())
        current = os.stat(path, follow_symlinks=False)
    identity = lambda value: (value.st_dev, value.st_ino, value.st_size, value.st_mtime_ns, value.st_ctime_ns)
    if len(raw) > 16 * 1024 * 1024 or identity(before) != identity(after) or identity(after) != identity(current):
        raise ValueError("catalog changed")
    result = catalog_references(json.loads(raw))
    result["sha256"] = hashlib.sha256(raw).hexdigest()
    return result


def catalog_references(data):
    result = {"projection_complete": False, "authority": False, "records": [], "errors": []}
    if not isinstance(data, dict) or type(data.get("version")) is not int or data["version"] != 13:
        result["errors"].append("unsupported-catalog-schema")
        return result
    result["version"] = 13
    fields = {
        "persistent_resources": ("id", "owner", "kind", "native_ref", "state", "workspace_id", "restore_source"),
        "base_assets": ("id", "owner", "native_ref", "state"),
        "workspace_leases": ("workspace_id", "environment_id", "owner", "instance_id", "runtime_ref", "state", "snapshot_source"),
        "snapshots": ("id", "state"),
    }
    def reference(value):
        value = text(value)
        # Native identifiers only; no URI, arbitrary configuration or source content.
        if value and not re.fullmatch(r"[A-Za-z0-9_.:/-]+", value):
            raise ValueError("unreportable reference")
        if "://" in value:
            raise ValueError("unreportable URI")
        return value
    for section, allowed in fields.items():
        rows = data.get(section, {})
        if not isinstance(rows, dict) or len(rows) > LIMIT:
            result["errors"].append(section)
            continue
        for index, (key, value) in enumerate(rows.items()):
            try:
                if not isinstance(value, dict):
                    raise ValueError("invalid catalog row")
                row = {"section": section, "key": reference(key), "review": "required"}
                for field in allowed:
                    if field in value:
                        row[field] = reference(value[field])
                if section == "snapshots":
                    components = value.get("components", [])
                    if not isinstance(components, list) or len(components) > LIMIT:
                        raise ValueError("invalid components")
                    row["components"] = []
                    for component in components:
                        if not isinstance(component, dict):
                            raise ValueError("invalid component")
                        row["components"].append({f: reference(component[f]) for f in ("role", "native_ref", "owner", "state")})
                if section == "workspace_leases" and "persistent_resource" in value:
                    attached = value["persistent_resource"]
                    if not isinstance(attached, dict):
                        raise ValueError("invalid attachment")
                    row["persistent_resource"] = {f: reference(attached[f]) for f in ("id", "owner") if f in attached}
                result["records"].append(row)
            except (ValueError, TypeError, KeyError):
                # Do not put the malformed backend value in diagnostics.
                result["errors"].append(section + ":row:" + str(index))
    result["projection_complete"] = not result["errors"]
    result["unreviewed"] = ["native ownership comparison", "repository catalog and Workspace source paths", "pending copies/restores and transient records", "settings, Policy and credentials", "manual and unregistered data"]
    return result


def main():
    if len(sys.argv) != 1 and not (len(sys.argv) == 3 and sys.argv[1] == "--catalog"):
        print("Usage: python3 tools/evacuation_inventory.py [--catalog environments.json]", file=sys.stderr)
        return 2
    try:
        report = inventory()
    except (ValueError, TypeError, KeyError):
        print("Invalid Incus inventory metadata; inventory incomplete", file=sys.stderr)
        return 1
    catalog_ok = True
    if len(sys.argv) == 3:
        try:
            report["catalog"] = catalog_inventory(sys.argv[2])
            catalog_ok = report["catalog"]["projection_complete"]
        except (ValueError, TypeError, KeyError, OSError):
            report["catalog"] = {"projection_complete": False, "authority": False, "errors": ["catalog-unavailable-or-changing"]}
            catalog_ok = False
    json.dump(report, sys.stdout, ensure_ascii=True, indent=2)
    print()
    return 0 if report["native_queries_complete"] and catalog_ok else 1


if __name__ == "__main__":
    sys.exit(main())