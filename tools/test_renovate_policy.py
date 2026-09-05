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

automerge_disabled = copy.deepcopy(base)
automerge_disabled["automerge"] = False
assert_invalid(automerge_disabled, "top-level automerge must be true", "global automerge disabled")

branch_automerge = copy.deepcopy(base)
branch_automerge["automergeType"] = "branch"
assert_invalid(branch_automerge, "automergeType must remain pr", "branch automerge")

age_removed = copy.deepcopy(base)
age_removed["minimumReleaseAge"] = "0 days"
assert_invalid(age_removed, "minimumReleaseAge must remain 30 days", "release age regression")

missing_timestamp = copy.deepcopy(base)
missing_timestamp["minimumReleaseAgeBehaviour"] = "timestamp-optional"
assert_invalid(
    missing_timestamp,
    "minimumReleaseAgeBehaviour must remain timestamp-required",
    "timestamp requirement regression",
)

non_strict_internal_checks = copy.deepcopy(base)
non_strict_internal_checks["internalChecksFilter"] = "flexible"
assert_invalid(
    non_strict_internal_checks,
    "internalChecksFilter must remain strict",
    "internal checks regression",
)

ignore_tests = copy.deepcopy(base)
ignore_tests["ignoreTests"] = True
assert_invalid(ignore_tests, "ignoreTests must remain false", "test gating disabled")

platform_automerge = copy.deepcopy(base)
platform_automerge["platformAutomerge"] = True
assert_invalid(
    platform_automerge,
    "platformAutomerge must remain false",
    "platform automerge regression",
)

manual_exception = copy.deepcopy(base)
manual_exception["packageRules"] = [
    {
        "description": "manual exception",
        "matchDepNames": ["example/dependency"],
        "automerge": False,
    }
]
assert_invalid(
    manual_exception,
    "disables automerge",
    "manual merge exception",
)

weaker_age_rule = copy.deepcopy(base)
weaker_age_rule["packageRules"] = [
    {
        "description": "weaker cooldown",
        "matchDepNames": ["example/dependency"],
        "minimumReleaseAge": "7 days",
    }
]
assert_invalid(
    weaker_age_rule,
    "overrides minimumReleaseAge away from 30 days",
    "package cooldown regression",
)

package_ignore_tests = copy.deepcopy(base)
package_ignore_tests["packageRules"] = [
    {
        "description": "skip checks",
        "matchDepNames": ["example/dependency"],
        "ignoreTests": True,
    }
]
assert_invalid(
    package_ignore_tests,
    "disables test gating via ignoreTests",
    "package test gating regression",
)

print("Renovate policy regression tests: OK")
