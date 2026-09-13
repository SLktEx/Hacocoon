#!/usr/bin/env python3
"""Bounded CI substrate observations; never archive whole product state/logs."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess

SCALAR = re.compile(r"[A-Za-z0-9_. /:+()-]{0,512}\Z")


def probe(command):
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=10)
        # stdout is accepted only for these fixed version/state commands. Drop
        # any unexpected bytes and every stderr; exit status remains actionable.
        output = result.stdout.strip()
        return {"exit_code": result.returncode,
                "value": output if SCALAR.fullmatch(output) else "omitted"}
    except subprocess.TimeoutExpired:
        return {"state": "timeout"}
    except OSError:
        return {"state": "unavailable"}


def inventory(command, fields):
    try:
        result = subprocess.run(command, capture_output=True, text=True, timeout=10)
        if result.returncode or len(result.stdout) > 1 << 20:
            return {"state": "unavailable", "exit_code": result.returncode}
        rows = json.loads(result.stdout)
        if not isinstance(rows, list) or len(rows) > 200:
            return {"state": "invalid"}
        # Only identifiers/state are retained, never expanded config, devices,
        # environment, image properties, SSH keys or controller journal bodies.
        selected = []
        for row in rows:
            selected.append({key: row[key] for key in fields if isinstance(row.get(key), str)
                             and re.fullmatch(r"[A-Za-z0-9_.-]{1,128}", row[key])})
        return {"state": "observed", "objects": selected}
    except (OSError, subprocess.TimeoutExpired, ValueError, TypeError, AttributeError):
        return {"state": "unavailable"}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    data = {"schema": 1, "sha": os.environ.get("GITHUB_SHA"),
            "run": os.environ.get("GITHUB_RUN_ID"), "attempt": os.environ.get("GITHUB_RUN_ATTEMPT"),
            "runner_image": {k: os.environ.get(k) for k in ("ImageOS", "ImageVersion", "RUNNER_OS", "RUNNER_ARCH")}}
    commands = {"kernel": ["uname", "-r"], "go": ["go", "version"]}
    if os.name == "posix":
        commands.update({
            "incus_client": ["incus", "--version"],
            "incus_package": ["dpkg-query", "-W", "-f=${Version}", "incus-base"],
            "incus_service": ["systemctl", "show", "--property=ActiveState", "--value", "incus.service"],
            "controller_service": ["systemctl", "show", "--property=ActiveState", "--value", "haco-controller.service"],
        })
    data["probes"] = {name: probe(command) for name, command in commands.items()}
    if os.name == "posix":
        data["inventory"] = {
            "instances": inventory(["incus", "list", "--all-projects", "--format=json"], ["name", "project", "status"]),
            "networks": inventory(["incus", "network", "list", "--project=default", "--format=json"], ["name", "type", "status"]),
            "storage": inventory(["incus", "storage", "list", "--format=json"], ["name", "driver", "status"]),
        }
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(data, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
