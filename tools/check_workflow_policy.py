#!/usr/bin/env python3
from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

APPROVED_RUNNERS = {"ubuntu-24.04"}
FULL_SHA_RE = re.compile(r"^[0-9a-fA-F]{40}$")
USES_RE = re.compile(r"^\s*(?:-\s*)?uses:\s*([^\s#]+)")
RUNS_ON_RE = re.compile(r"^\s*runs-on:\s*([^#]+?)\s*$")
ACTIVE_TRUE = r'["\']?(?:1|true|yes|on)["\']?'
PRIVILEGED_ENV_RE = re.compile(
    rf"\b(HACO_E2E_INCUS|HACO_EXPERIMENTAL_EC2)\s*[:=]\s*{ACTIVE_TRUE}\b",
    re.IGNORECASE,
)


@dataclass(frozen=True)
class Violation:
    path: Path
    line: int
    message: str

    def render(self) -> str:
        return f"ERROR {self.path}:{self.line}: {self.message}"


def _indent(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def _is_ignored(line: str) -> bool:
    stripped = line.strip()
    return not stripped or stripped.startswith("#")


def _has_yaml_key(lines: list[str], key: str) -> bool:
    pattern = re.compile(rf"^\s*{re.escape(key)}\s*:")
    return any(pattern.match(line) for line in lines if not _is_ignored(line))


def _step_block(lines: list[str], index: int) -> list[str]:
    base_indent = _indent(lines[index])
    block = [lines[index]]
    for line in lines[index + 1 :]:
        if _is_ignored(line):
            block.append(line)
            continue
        if _indent(line) <= base_indent and line.lstrip().startswith("-"):
            break
        if _indent(line) < base_indent:
            break
        block.append(line)
    return block


def _check_permissions(path: Path, lines: list[str], violations: list[Violation]) -> None:
    for index, line in enumerate(lines):
        match = re.match(r"^(\s*)permissions:\s*([^#]*?)\s*(?:#.*)?$", line)
        if not match:
            continue
        base_indent = len(match.group(1))
        inline = match.group(2).strip()
        if inline:
            if inline not in {"read-all", "{}"}:
                violations.append(
                    Violation(path, index + 1, f"PR workflow permissions must be read-only, got {inline!r}")
                )
            continue
        for offset in range(index + 1, len(lines)):
            child = lines[offset]
            if _is_ignored(child):
                continue
            if _indent(child) <= base_indent:
                break
            item = re.match(r"^\s*([A-Za-z0-9_-]+):\s*([^#]+?)\s*(?:#.*)?$", child)
            if not item:
                continue
            scope, value = item.groups()
            value = value.strip()
            if value not in {"read", "none"}:
                violations.append(
                    Violation(
                        path,
                        offset + 1,
                        f"PR workflow permission {scope!r} must be read/none, got {value!r}",
                    )
                )


def check_text(path: Path, text: str) -> list[Violation]:
    lines = text.splitlines()
    violations: list[Violation] = []
    is_pr = _has_yaml_key(lines, "pull_request")

    for forbidden in ("pull_request_target", "workflow_run"):
        for index, line in enumerate(lines):
            if _is_ignored(line):
                continue
            if re.match(rf"^\s*{forbidden}\s*:", line):
                violations.append(Violation(path, index + 1, f"forbidden workflow trigger: {forbidden}"))

    for index, line in enumerate(lines):
        runner = RUNS_ON_RE.match(line)
        if runner:
            value = runner.group(1).strip().strip("\"'")
            if value not in APPROVED_RUNNERS:
                violations.append(
                    Violation(
                        path,
                        index + 1,
                        f"runner {value!r} is not approved; allowed: {', '.join(sorted(APPROVED_RUNNERS))}",
                    )
                )

        use = USES_RE.match(line)
        if not use:
            continue
        target = use.group(1)
        if target.startswith("./"):
            continue
        if "@" not in target:
            violations.append(Violation(path, index + 1, f"external action must use an immutable SHA: {target}"))
            continue
        action, ref = target.rsplit("@", 1)
        if not FULL_SHA_RE.fullmatch(ref):
            violations.append(
                Violation(path, index + 1, f"external action {action} must be pinned to a full commit SHA")
            )

        block_text = "\n".join(_step_block(lines, index))
        if is_pr and action == "actions/checkout":
            if not re.search(r"(?m)^\s*persist-credentials:\s*false\s*(?:#.*)?$", block_text):
                violations.append(
                    Violation(path, index + 1, "actions/checkout in PR workflow requires persist-credentials: false")
                )
        if is_pr and action == "actions/cache":
            violations.append(Violation(path, index + 1, "actions/cache is not permitted in untrusted PR workflows"))
        if is_pr and action == "actions/setup-go":
            if not re.search(r"(?m)^\s*cache:\s*false\s*(?:#.*)?$", block_text):
                violations.append(
                    Violation(path, index + 1, "actions/setup-go in PR workflow requires cache: false")
                )
        if is_pr and action == "actions/download-artifact":
            if re.search(r"(?m)^\s*(?:run-id|github-token|repository):", block_text):
                violations.append(
                    Violation(path, index + 1, "cross-run/external artifact download is not permitted in PR workflows")
                )

    if is_pr:
        _check_permissions(path, lines, violations)
        secret_match = re.search(r"\$\{\{\s*secrets\.", text)
        if secret_match:
            line = text[: secret_match.start()].count("\n") + 1
            violations.append(Violation(path, line, "repository/environment secrets are not permitted in PR workflows"))
        for match in PRIVILEGED_ENV_RE.finditer(text):
            line = text[: match.start()].count("\n") + 1
            violations.append(Violation(path, line, f"{match.group(1)} must remain disabled in normal PR CI"))

    return violations


def check_workflow_file(path: Path) -> list[Violation]:
    return check_text(path, path.read_text(encoding="utf-8"))


def discover(root: Path) -> list[Path]:
    return sorted([*root.glob("*.yml"), *root.glob("*.yaml")])


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Validate Hacocoon GitHub Actions trust boundaries.")
    parser.add_argument(
        "workflow_dir",
        nargs="?",
        type=Path,
        default=Path(".github/workflows"),
        help="workflow directory (default: .github/workflows)",
    )
    args = parser.parse_args(argv)

    files = discover(args.workflow_dir)
    if not files:
        print(f"ERROR {args.workflow_dir}: no workflow files found", file=sys.stderr)
        return 2

    violations = [violation for path in files for violation in check_workflow_file(path)]
    if violations:
        for violation in violations:
            print(violation.render(), file=sys.stderr)
        return 1

    print(f"workflow trust policy: OK ({len(files)} files)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
