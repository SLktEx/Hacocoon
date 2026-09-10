#!/usr/bin/env python3
"""Read-only Incus inventory for manual evacuation planning, not a backup."""
import json
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


def main():
    if len(sys.argv) != 1:
        print("Usage: python3 tools/evacuation_inventory.py", file=sys.stderr)
        return 2
    try:
        report = inventory()
    except (ValueError, TypeError, KeyError):
        print("Invalid Incus inventory metadata; inventory incomplete", file=sys.stderr)
        return 1
    json.dump(report, sys.stdout, ensure_ascii=True, indent=2)
    print()
    return 0 if report["native_queries_complete"] else 1


if __name__ == "__main__":
    sys.exit(main())