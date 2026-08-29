#!/usr/bin/env python3
from __future__ import annotations

import copy
import importlib.util
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "tools/check_renovate_policy.py"
spec = importlib.util.spec_from_file_location("check_renovate_policy", MODULE_PATH)
assert spec is not None and spec.loader is not None
policy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(policy)

base = policy.load(ROOT / "renovate.json")


def assert_valid(config, label: str) -> None:
    errors = policy.validate(config)
    if errors:
        raise AssertionError(f"{label} unexpectedly invalid: {errors}")


def assert_invalid(config, expected: str, label: str) -> None:
    errors = policy.validate(config)
    if not any(expected in error for error in errors):
        raise AssertionError(f"{label} did not fail for {expected!r}: {errors}")


assert_valid(base, "current config")

unsafe_global = copy.deepcopy(base)
unsafe_global["automerge"] = True
assert_invalid(unsafe_global, "top-level automerge must be false", "global automerge")

future_executable_manager = copy.deepcopy(base)
future_executable_manager["packageRules"].append(
    {
        "description": "future executable dependency manager",
        "matchManagers": ["custom.regex"],
        "automerge": True,
    }
)
assert_invalid(
    future_executable_manager,
    "without an explicit non-empty matchDepNames allowlist",
    "new executable manager",
)

unknown_exact_package = copy.deepcopy(base)
unknown_exact_package["packageRules"].append(
    {
        "description": "unknown build tool",
        "matchDepNames": ["example/build-tool"],
        "automerge": True,
    }
)
assert_invalid(
    unknown_exact_package,
    "not in the audited package allowlist",
    "unknown exact package",
)

broad_manager_even_if_named = copy.deepcopy(base)
broad_manager_even_if_named["packageRules"].append(
    {
        "description": "broad executable class",
        "matchManagers": ["github-actions"],
        "matchDepNames": ["actions/example"],
        "automerge": True,
    }
)
assert_invalid(
    broad_manager_even_if_named,
    "manager/datasource/pattern-wide",
    "broad manager opt-in",
)

age_removed = copy.deepcopy(base)
age_removed["minimumReleaseAge"] = "0 days"
assert_invalid(age_removed, "minimumReleaseAge must remain 30 days", "release age regression")

print("Renovate policy regression tests: OK")
