#!/usr/bin/env python3
from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

# There are intentionally no automerge-safe dependency identities today.
# Future opt-ins must add an exact package identity here and a matching narrowly
# scoped packageRule in renovate.json. Manager-wide or wildcard automerge is
# deliberately forbidden because new executable dependencies must fail closed.
AUTOMERGE_PACKAGE_ALLOWLIST: frozenset[str] = frozenset()


def validate(config: dict[str, Any]) -> list[str]:
    errors: list[str] = []

    if config.get("automerge") is not False:
        errors.append("top-level automerge must be false")
    if config.get("minimumReleaseAge") != "30 days":
        errors.append("minimumReleaseAge must remain 30 days")
    if config.get("minimumReleaseAgeBehaviour") != "timestamp-required":
        errors.append("minimumReleaseAgeBehaviour must remain timestamp-required")
    if config.get("internalChecksFilter") != "strict":
        errors.append("internalChecksFilter must remain strict")
    if config.get("platformAutomerge") is not False:
        errors.append("platformAutomerge must remain false")

    rules = config.get("packageRules", [])
    if not isinstance(rules, list):
        return errors + ["packageRules must be a list"]

    for index, rule in enumerate(rules):
        if not isinstance(rule, dict):
            errors.append(f"packageRules[{index}] must be an object")
            continue
        if rule.get("automerge") is not True:
            continue

        dep_names = rule.get("matchDepNames")
        if not isinstance(dep_names, list) or not dep_names:
            errors.append(
                f"packageRules[{index}] enables automerge without an explicit non-empty matchDepNames allowlist"
            )
            continue
        if any(not isinstance(name, str) or name not in AUTOMERGE_PACKAGE_ALLOWLIST for name in dep_names):
            errors.append(
                f"packageRules[{index}] automerges a dependency not in the audited package allowlist: {dep_names!r}"
            )
        if any(key in rule for key in ("matchManagers", "matchDatasources", "matchPackagePatterns")):
            errors.append(
                f"packageRules[{index}] automerge must not be enabled by manager/datasource/pattern-wide matching"
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

    print("RENOVATE POLICY OK: automerge is fail-closed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
