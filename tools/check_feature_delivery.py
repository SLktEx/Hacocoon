#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
VERSION_RE = re.compile(r"^0\.(\d+)$")
FEATURE_TITLE_RE = re.compile(r"^feat(?:\([^)]+\))?!?:\s+", re.IGNORECASE)

FEATURE_REQUIRED_CHANGES = (
    "README.md",
    "README.ja.md",
    "CODEX_START_HERE.md",
    "docs/README.md",
    "docs/README.ja.md",
    "docs/00_REBASELINE_AND_ROADMAP.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.ja.md",
    "docs/IMPLEMENTATION_STATUS.md",
    "docs/IMPLEMENTATION_STATUS.ja.md",
    "docs/90_CODEX_IMPLEMENTATION_HANDOFF.md",
)

CURRENT_REQUIRED = (
    "docs/FEATURE_DELIVERY.md",
    "docs/00D_VERSIONING_AND_RELEASE_STATUS.md",
    "docs/IMPLEMENTATION_STATUS.md",
)


def fail(message: str) -> None:
    print(f"FEATURE DELIVERY ERROR: {message}", file=sys.stderr)


def read_version(path: Path | None = None) -> str:
    path = path or ROOT / "VERSION"
    if not path.is_file():
        raise ValueError("missing VERSION file")
    value = path.read_text(encoding="utf-8").strip()
    if not VERSION_RE.fullmatch(value):
        raise ValueError(f"VERSION must be a numeric pre-1.0 milestone like 0.17, got {value!r}")
    return value


def validate_repository(version: str) -> list[str]:
    errors: list[str] = []
    marker = f"v{version}"

    for rel in CURRENT_REQUIRED:
        if not (ROOT / rel).is_file():
            errors.append(f"missing feature-delivery source-of-truth file: {rel}")

    numbering = ROOT / "docs/00D_VERSIONING_AND_RELEASE_STATUS.md"
    if numbering.is_file() and marker not in numbering.read_text(encoding="utf-8"):
        errors.append(f"numbering document does not mention current VERSION milestone {marker}")

    status = ROOT / "docs/IMPLEMENTATION_STATUS.md"
    if status.is_file() and marker not in status.read_text(encoding="utf-8"):
        errors.append(f"implementation status does not mention current VERSION milestone {marker}")

    return errors


def git(*args: str, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        ["git", *args],
        cwd=ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=check,
    )


def ensure_base_available(base: str) -> None:
    result = git("cat-file", "-e", f"{base}^{{commit}}", check=False)
    if result.returncode != 0:
        raise ValueError(
            f"base ref {base!r} is not available locally; checkout/fetch enough history or pass a reachable base"
        )


def version_at(base: str) -> str | None:
    result = git("show", f"{base}:VERSION", check=False)
    if result.returncode != 0:
        return None
    value = result.stdout.strip()
    if not VERSION_RE.fullmatch(value):
        raise ValueError(f"{base}:VERSION has invalid value {value!r}")
    return value


def changed_files(base: str) -> set[str]:
    result = git("diff", "--name-only", "--diff-filter=ACMRT", base, "HEAD")
    return {line.strip() for line in result.stdout.splitlines() if line.strip()}


def validate_increment(old: str, new: str) -> list[str]:
    old_match = VERSION_RE.fullmatch(old)
    new_match = VERSION_RE.fullmatch(new)
    if old_match is None or new_match is None:
        return [f"invalid VERSION transition: {old!r} -> {new!r}"]
    expected = int(old_match.group(1)) + 1
    actual = int(new_match.group(1))
    if actual != expected:
        return [f"feature milestone must increment exactly once: {old} -> 0.{expected}, got {new}"]
    return []


def validate_feature_diff(base: str, version: str, feature: bool) -> list[str]:
    errors: list[str] = []
    ensure_base_available(base)
    old_version = version_at(base)
    changes = changed_files(base)

    # VERSION is introduced after v0.17 was already documented/implemented.
    # The initial marker is policy bootstrap rather than another product feature.
    if old_version is None:
        if feature:
            errors.append("feature delivery cannot be validated against a base that has no VERSION marker")
        return errors

    version_changed = "VERSION" in changes and old_version != version

    if feature and not version_changed:
        errors.append(
            "feature PR did not advance VERSION; every independently useful product feature consumes one numeric milestone"
        )
        return errors

    if not version_changed:
        return errors

    if not feature:
        errors.append("VERSION changed but the PR title is not feat: / feat(scope):")

    errors.extend(validate_increment(old_version, version))

    missing = [path for path in FEATURE_REQUIRED_CHANGES if path not in changes]
    if missing:
        errors.append(
            "VERSION changed without updating the complete feature documentation set: " + ", ".join(missing)
        )

    spec_re = re.compile(rf"^docs/\d+_v{re.escape(version)}_[^/]+\.md$")
    if not any(spec_re.match(path) for path in changes):
        errors.append(
            f"VERSION advanced to {version}, but no owning versioned specification "
            f"(docs/NN_v{version}_*.md) was added or updated"
        )

    return errors


def github_pr_context() -> tuple[str | None, str | None]:
    if os.environ.get("GITHUB_EVENT_NAME") != "pull_request":
        return None, None
    event_path = os.environ.get("GITHUB_EVENT_PATH")
    if not event_path:
        return None, None
    try:
        event = json.loads(Path(event_path).read_text(encoding="utf-8"))
        pull = event["pull_request"]
        return pull["base"]["sha"], pull.get("title")
    except (OSError, KeyError, TypeError, json.JSONDecodeError):
        return None, None


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Keep Hacocoon feature implementation, milestone VERSION, and documentation in one atomic delivery."
    )
    parser.add_argument("--base", help="base ref/commit used to validate the feature-delivery diff")
    parser.add_argument("--feature", action="store_true", help="require this change to advance the feature milestone")
    parser.add_argument("--pr-title", help="PR title; feat: / feat(scope): automatically enables feature mode")
    args = parser.parse_args()

    try:
        version = read_version()
    except ValueError as exc:
        fail(str(exc))
        return 1

    errors = validate_repository(version)
    event_base, event_title = github_pr_context()
    base = args.base or event_base
    title = args.pr_title or event_title or ""
    feature = args.feature or bool(FEATURE_TITLE_RE.match(title))

    if base:
        try:
            errors.extend(validate_feature_diff(base, version, feature))
        except (ValueError, subprocess.CalledProcessError) as exc:
            errors.append(str(exc))
    elif feature:
        errors.append("feature mode requires --base (or a GitHub pull_request event with base SHA)")

    if errors:
        for error in errors:
            fail(error)
        return 1

    detail = f"current milestone v{version}"
    if base:
        detail += f"; diff checked against {base}"
    if feature:
        detail += "; feature milestone bump required"
    print(f"FEATURE DELIVERY OK: {detail}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
