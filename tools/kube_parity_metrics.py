#!/usr/bin/env python3
"""Report branch-local complexity evidence for the Kubernetes parity experiment.

This intentionally does not produce a winner. It reports Hacocoon-owned physical
source size for provider-specific and shared security paths so the final parity
comparison can be based on measurements instead of architecture preference.
"""

from __future__ import annotations

import argparse
import json
from pathlib import Path
from typing import Iterable

ROOT = Path(__file__).resolve().parents[1]
SOURCE_SUFFIXES = {".go", ".py", ".sh", ".ps1"}

GROUPS = {
    "incus_provider_impl": {
        "roots": ["modules/runtime/incus"],
        "tests": False,
    },
    "incus_provider_tests": {
        "roots": ["modules/runtime/incus"],
        "tests": True,
    },
    "kubernetes_provider_impl": {
        "roots": ["modules/runtime/kubernetes"],
        "tests": False,
    },
    "kubernetes_provider_tests": {
        "roots": ["modules/runtime/kubernetes"],
        "tests": True,
    },
    "shared_egress_impl": {
        "roots": ["internal/egress", "modules/standard/egressproxy"],
        "tests": False,
    },
    "shared_egress_tests": {
        "roots": ["internal/egress", "modules/standard/egressproxy"],
        "tests": True,
    },
}


def is_test(path: Path) -> bool:
    name = path.name
    return name.endswith("_test.go") or "/test/" in path.as_posix() or name.startswith("test_")


def source_files(roots: Iterable[str], tests: bool) -> list[Path]:
    result: list[Path] = []
    for raw_root in roots:
        root = ROOT / raw_root
        if not root.exists():
            continue
        for path in sorted(root.rglob("*")):
            if not path.is_file() or path.suffix not in SOURCE_SUFFIXES:
                continue
            if is_test(path) != tests:
                continue
            result.append(path)
    return result


def measure(paths: list[Path]) -> dict[str, object]:
    physical = 0
    nonblank = 0
    by_language: dict[str, int] = {}
    relative: list[str] = []
    for path in paths:
        text = path.read_text(encoding="utf-8")
        lines = text.splitlines()
        physical += len(lines)
        nonblank += sum(1 for line in lines if line.strip())
        by_language[path.suffix.lstrip(".")] = by_language.get(path.suffix.lstrip("."), 0) + len(lines)
        relative.append(path.relative_to(ROOT).as_posix())
    return {
        "files": len(paths),
        "physical_lines": physical,
        "nonblank_lines": nonblank,
        "physical_lines_by_language": dict(sorted(by_language.items())),
        "paths": relative,
    }


def collect() -> dict[str, object]:
    return {
        "scope": "main-kube branch-local evidence; no merge recommendation implied",
        "groups": {
            name: measure(source_files(config["roots"], bool(config["tests"])))
            for name, config in GROUPS.items()
        },
        "interpretation": [
            "Provider LOC is only Hacocoon-owned complexity; it does not include the external Kubernetes/CNI/CSI/RuntimeClass stack.",
            "Shared egress code is reported separately because exact domain-aware authorization may remain required on either runtime.",
            "A smaller provider is not full-system simplicity unless feature parity and operational dependency counts are also equivalent.",
        ],
    }


def print_table(report: dict[str, object]) -> None:
    groups = report["groups"]
    print("group\tfiles\tphysical\tnonblank")
    for name, measurement in groups.items():
        print(
            f"{name}\t{measurement['files']}\t{measurement['physical_lines']}\t{measurement['nonblank_lines']}"
        )
    print()
    for note in report["interpretation"]:
        print(f"- {note}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--json", action="store_true", help="emit JSON instead of a compact table")
    args = parser.parse_args()
    report = collect()
    if args.json:
        print(json.dumps(report, indent=2, sort_keys=True))
    else:
        print_table(report)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
