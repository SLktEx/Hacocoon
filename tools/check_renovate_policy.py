#!/usr/bin/env python3
from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any


def validate(config: dict[str, Any]) -> list[str]:
    errors: list[str] = []

    if config.get("automerge") is not True:
        errors.append("top-level automerge must be true")
    if config.get("automergeType") != "pr":
        errors.append("automergeType must remain pr")
    if config.get("minimumReleaseAge") != "30 days":
        errors.append("minimumReleaseAge must remain 30 days")
    if config.get("minimumReleaseAgeBehaviour") != "timestamp-required":
        errors.append("minimumReleaseAgeBehaviour must remain timestamp-required")
    if config.get("internalChecksFilter") != "strict":
        errors.append("internalChecksFilter must remain strict")
    if config.get("ignoreTests") is not False:
        errors.append("ignoreTests must remain false")
    if config.get("platformAutomerge") is not False:
        errors.append("platformAutomerge must remain false")

    rules = config.get("packageRules", [])
    if not isinstance(rules, list):
        return errors + ["packageRules must be a list"]

    for index, rule in enumerate(rules):
        if not isinstance(rule, dict):
            errors.append(f"packageRules[{index}] must be an object")
            continue
        if rule.get("automerge") is False:
            errors.append(
                f"packageRules[{index}] disables automerge; all dependency updates must follow the global cooldown-first automerge policy"
            )
        if rule.get("minimumReleaseAge") not in (None, "30 days"):
            errors.append(
                f"packageRules[{index}] overrides minimumReleaseAge away from 30 days"
            )
        if rule.get("minimumReleaseAgeBehaviour") not in (None, "timestamp-required"):
            errors.append(
                f"packageRules[{index}] overrides minimumReleaseAgeBehaviour away from timestamp-required"
            )
        if rule.get("internalChecksFilter") not in (None, "strict"):
            errors.append(
                f"packageRules[{index}] overrides internalChecksFilter away from strict"
            )
        if rule.get("ignoreTests") is True:
            errors.append(
                f"packageRules[{index}] disables test gating via ignoreTests"
            )

    return errors


def load(path: Path) -> dict[str, Any]:
    value = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(value, dict):
        raise ValueError("Renovate config root must be an object")
    return value


def main(argv: list[str] | None = None) -> int:
    args = sys.argv[1:] if argv is None else argv
    path = Path(args[0]) if args else Path("renovate.json")
    try:
        config = load(path)
    except (OSError, json.JSONDecodeError, ValueError) as exc:
        print(f"RENOVATE POLICY FAILED: {exc}", file=sys.stderr)
        return 2

    errors = validate(config)
    if errors:
        print("RENOVATE POLICY FAILED", file=sys.stderr)
        for error in errors:
            print(f"ERROR: {error}", file=sys.stderr)
        return 1

    print("RENOVATE POLICY OK: 30-day cooldown, required checks, then automerge")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
