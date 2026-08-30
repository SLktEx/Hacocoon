#!/usr/bin/env python3
from __future__ import annotations

import importlib.util
from pathlib import Path
import subprocess
import tempfile

SCRIPT = Path(__file__).with_name("check_feature_delivery.py")
spec = importlib.util.spec_from_file_location("check_feature_delivery", SCRIPT)
assert spec and spec.loader
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


def run(repo: Path, *args: str) -> None:
    subprocess.run(["git", *args], cwd=repo, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)


def write_sources(repo: Path, version: str) -> None:
    (repo / "VERSION").write_text(version + "\n", encoding="utf-8")
    marker = f"v{version}"
    for rel in mod.FEATURE_REQUIRED_CHANGES:
        path = repo / rel
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(f"current {marker}\n", encoding="utf-8")
    (repo / "docs/FEATURE_DELIVERY.md").write_text("feature delivery contract\n", encoding="utf-8")


def commit_all(repo: Path, message: str) -> str:
    run(repo, "add", ".")
    run(repo, "commit", "-m", message)
    return subprocess.run(["git", "rev-parse", "HEAD"], cwd=repo, check=True, stdout=subprocess.PIPE, text=True).stdout.strip()


def new_repo() -> tuple[Path, tempfile.TemporaryDirectory[str], str]:
    temp = tempfile.TemporaryDirectory()
    repo = Path(temp.name)
    run(repo, "init", "-q")
    run(repo, "config", "user.email", "test@example.invalid")
    run(repo, "config", "user.name", "test")
    write_sources(repo, "0.17")
    (repo / "docs/17_v0.17_EXISTING.md").write_text("v0.17\n", encoding="utf-8")
    base = commit_all(repo, "base")
    return repo, temp, base


def test_feature_requires_bump() -> None:
    repo, temp, base = new_repo()
    try:
        mod.ROOT = repo
        (repo / "cmd").mkdir()
        (repo / "cmd/new.go").write_text("package cmd\n", encoding="utf-8")
        commit_all(repo, "feature without version")
        errors = mod.validate_feature_diff(base, mod.read_version(), True)
        assert any("did not advance VERSION" in e for e in errors), errors
    finally:
        temp.cleanup()


def test_bump_requires_docs_and_spec() -> None:
    repo, temp, base = new_repo()
    try:
        mod.ROOT = repo
        (repo / "VERSION").write_text("0.18\n", encoding="utf-8")
        commit_all(repo, "incomplete feature")
        errors = mod.validate_feature_diff(base, mod.read_version(), True)
        assert any("complete feature documentation set" in e for e in errors), errors
        assert any("no owning versioned specification" in e for e in errors), errors
    finally:
        temp.cleanup()


def test_complete_feature_passes() -> None:
    repo, temp, base = new_repo()
    try:
        mod.ROOT = repo
        write_sources(repo, "0.18")
        (repo / "docs/18_v0.18_NEW_FEATURE.md").write_text("v0.18\n", encoding="utf-8")
        commit_all(repo, "complete feature")
        assert mod.validate_repository(mod.read_version()) == []
        assert mod.validate_feature_diff(base, mod.read_version(), True) == []
    finally:
        temp.cleanup()


def test_non_feature_without_bump_passes() -> None:
    repo, temp, base = new_repo()
    try:
        mod.ROOT = repo
        (repo / "notes.txt").write_text("fix\n", encoding="utf-8")
        commit_all(repo, "fix")
        assert mod.validate_feature_diff(base, mod.read_version(), False) == []
    finally:
        temp.cleanup()


def test_version_bump_requires_feat_title_mode() -> None:
    repo, temp, base = new_repo()
    try:
        mod.ROOT = repo
        write_sources(repo, "0.18")
        (repo / "docs/18_v0.18_NEW_FEATURE.md").write_text("v0.18\n", encoding="utf-8")
        commit_all(repo, "mislabeled feature")
        errors = mod.validate_feature_diff(base, mod.read_version(), False)
        assert any("PR title is not feat" in e for e in errors), errors
    finally:
        temp.cleanup()


def test_increment_exactly_one() -> None:
    assert mod.validate_increment("0.17", "0.18") == []
    assert mod.validate_increment("0.17", "0.19")
    assert mod.validate_increment("0.17", "0.17")


def test_version_bootstrap_is_not_feature() -> None:
    repo, temp, _ = new_repo()
    try:
        mod.ROOT = repo
        run(repo, "rm", "VERSION")
        base = commit_all(repo, "base without marker")
        (repo / "VERSION").write_text("0.17\n", encoding="utf-8")
        commit_all(repo, "bootstrap marker")
        assert mod.validate_feature_diff(base, mod.read_version(), False) == []
    finally:
        temp.cleanup()


def main() -> None:
    tests = [
        test_feature_requires_bump,
        test_bump_requires_docs_and_spec,
        test_complete_feature_passes,
        test_non_feature_without_bump_passes,
        test_version_bump_requires_feat_title_mode,
        test_increment_exactly_one,
        test_version_bootstrap_is_not_feature,
    ]
    for test in tests:
        test()
        print(f"PASS {test.__name__}")
    print("FEATURE DELIVERY TESTS OK")


if __name__ == "__main__":
    main()
