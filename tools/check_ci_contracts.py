#!/usr/bin/env python3
"""Fail closed if a maintained PR contract can disappear through routing."""
import json
from pathlib import Path
import re
import sys

from check_workflow_policy import _parse_tree, _materialize, _walk

ROOT = Path(__file__).resolve().parent.parent


def child(node, key):
    return next((n for n in node.children if n.key == key), None)


def value(node, key):
    found = child(node, key)
    return _materialize(found) if found else None


def step_name(node):
    return node.value if node.key == "name" else value(node, "name")


def check(text, spec):
    errors = []
    tree = _parse_tree(text)
    triggers = value(tree, "on") or {}
    pr = triggers.get("pull_request") if isinstance(triggers, dict) else None
    if not isinstance(triggers, dict) or "pull_request" not in triggers:
        errors.append("required workflow must run on pull_request")
    if pr and (set(pr) != {"branches"} or "main" not in pr["branches"]):
        errors.append("PR routing must include main and must not filter paths/types")
    jobs = child(tree, "jobs")
    variants = spec.get("job_names", [])
    if len(set(variants)) != len(variants) or {n.split(" (")[0] for n in variants} != set(spec["jobs"]):
        errors.append("required job variant inventory is incomplete or duplicated")
    for name in spec["jobs"]:
        job = child(jobs, name) if jobs else None
        if job is None:
            errors.append(f"missing required job {name}")
            continue
        if child(job, "if") or value(job, "continue-on-error") not in (None, False):
            errors.append(f"required job {name} may not be skipped or tolerated")
        steps = child(job, "steps")
        required_steps = spec.get("steps", {}).get(name, [])
        if not required_steps:
            errors.append(f"{name}: required step inventory is empty")
        for required in required_steps:
            matches = [s for s in steps.children if step_name(s) == required] if steps else []
            if len(matches) != 1 or child(matches[0], "if"):
                errors.append(f"{name}: required step missing, duplicated or conditional: {required}")
        for node in _walk(job):
            if node.key == "continue-on-error" and node.value is not False:
                # Only diagnostics may be best-effort; never authoritative actions.
                if value(node.parent, "if") != "failure()":
                    errors.append(f"{name}: required step failure is tolerated")
            if node.key == "runs-on" and node.value not in ("ubuntu-26.04", "windows-2025"):
                errors.append(f"{name}: runner OS must be explicit")
            if node.key == "check-latest" and node.value is not False:
                errors.append(f"{name}: dynamically selected toolchain")
            if node.key == "go-version" and isinstance(node.value, str) and "${{" not in node.value:
                if not re.fullmatch(r"1\.\d+\.\d+", node.value):
                    errors.append(f"{name}: Go must use an exact patch")
    evidence = child(jobs, value(tree, "name") + "-evidence") if jobs else None
    if evidence is None:
        errors.append("missing mandatory evidence job")
    else:
        if set(value(evidence, "needs") or []) != set(spec["jobs"]):
            errors.append("evidence must depend on every required job")
        if value(evidence, "if") != "always()":
            errors.append("evidence must inspect failed/skipped jobs with always()")
        if value(evidence, "continue-on-error") not in (None, False):
            errors.append("evidence failure cannot be tolerated")
        steps = child(evidence, "steps")
        gates = [s for s in steps.children if "python3 tools/ci_history.py --gate" in str(value(s, "run"))] if steps else []
        if len(gates) != 1 or child(gates[0], "if") or value(gates[0], "continue-on-error") not in (None, False):
            errors.append("evidence must enforce history")
    return errors


def main():
    specs = json.loads((ROOT / "tools/ci_contracts.json").read_text())
    errors = []
    for name, spec in specs.items():
        path = ROOT / ".github/workflows" / spec["file"]
        try:
            errors.extend(f"{name}: {e}" for e in check(path.read_text(), spec))
        except (OSError, ValueError, TypeError, AttributeError) as exc:
            errors.append(f"{name}: invalid workflow ({type(exc).__name__})")
    for error in errors:
        print(error, file=sys.stderr)
    return bool(errors)


if __name__ == "__main__":
    sys.exit(main())
