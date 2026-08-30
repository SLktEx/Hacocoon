#!/usr/bin/env python3
"""Black-box regression tests for tools/bump-milestone.

The suite copies the checked-out repository into temporary directories and runs
exactly the same wrapper maintainers use. The real checkout is never mutated.
"""

from pathlib import Path
import json
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "tools"))

from checkpoint_source import parse_checkpoint_source  # noqa: E402

MIRROR_PATHS = (
    "docs/status/checkpoints.yaml",
    "docs/status/versioning-and-release-status.md",
    "docs/status/versioning-and-release-status.ja.md",
    "docs/IMPLEMENTATION_STATUS.md",
    "docs/IMPLEMENTATION_STATUS.ja.md",
    "internal/buildinfo/checkpoint_generated.go",
)


def copy_repo(destination: Path) -> Path:
    repo = destination / "repo"
    shutil.copytree(
        ROOT,
        repo,
        ignore=shutil.ignore_patterns(
            ".git",
            "dist",
            "__pycache__",
            ".pytest_cache",
            ".mypy_cache",
        ),
    )
    return repo


def run(repo: Path, *argv: str) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        list(argv),
        cwd=repo,
        text=True,
        capture_output=True,
        check=False,
    )


class BumpMilestoneBlackBoxTest(unittest.TestCase):
    def setUp(self):
        self.tempdir = tempfile.TemporaryDirectory(prefix="haco-bump-milestone-")
        self.repo = copy_repo(Path(self.tempdir.name))
        source = parse_checkpoint_source(self.repo / "docs/status/checkpoints.yaml")
        self.current = source.current
        self.current_minor = int(self.current.removeprefix("v0."))
        self.next = f"v0.{self.current_minor + 1}"
        self.skipped = f"v0.{self.current_minor + 2}"
        self.gate = "Black-box Test Gate"

    def tearDown(self):
        self.tempdir.cleanup()

    def test_real_wrapper_advances_all_mirrors_and_runtime_identity(self):
        result = run(self.repo, "tools/bump-milestone", self.next, self.gate)
        output = result.stdout + result.stderr
        self.assertEqual(result.returncode, 0, output)
        self.assertIn(f"checkpoint advanced to {self.next}", output)
        self.assertIn("DOC CONSISTENCY OK", output)

        source = parse_checkpoint_source(self.repo / "docs/status/checkpoints.yaml")
        self.assertEqual(source.current, self.next)
        self.assertEqual(source.milestones[-1].version, self.next)
        self.assertEqual(source.milestones[-1].gate, self.gate)

        for relative in (
            "docs/status/versioning-and-release-status.md",
            "docs/status/versioning-and-release-status.ja.md",
        ):
            text = (self.repo / relative).read_text()
            self.assertIn(f"| {self.next} | {self.gate} |", text)

        current_expectations = {
            "docs/status/versioning-and-release-status.md": f"current milestone position is **{self.next}**",
            "docs/status/versioning-and-release-status.ja.md": f"現在のmilestone位置は **{self.next}**",
            "docs/IMPLEMENTATION_STATUS.md": f"current milestone position is **{self.next}**",
            "docs/IMPLEMENTATION_STATUS.ja.md": f"現在のmilestone位置は **{self.next}**",
        }
        for relative, expected in current_expectations.items():
            self.assertIn(expected, (self.repo / relative).read_text())

        generated = (self.repo / "internal/buildinfo/checkpoint_generated.go").read_text()
        self.assertIn(f'const GeneratedCheckpoint = "{self.next}"', generated)

        checker = run(self.repo, sys.executable, "tools/check_docs.py")
        self.assertEqual(checker.returncode, 0, checker.stdout + checker.stderr)
        self.assertIn("DOC CONSISTENCY OK", checker.stdout)

        version = run(self.repo, "go", "run", "./cmd/haco", "version", "--json")
        self.assertEqual(version.returncode, 0, version.stdout + version.stderr)
        identity = json.loads(version.stdout)
        self.assertEqual(identity["checkpoint"], self.next)

    def test_skipped_checkpoint_is_rejected_without_mutation(self):
        before = {relative: (self.repo / relative).read_bytes() for relative in MIRROR_PATHS}

        result = run(self.repo, "tools/bump-milestone", self.skipped, self.gate)
        output = result.stdout + result.stderr
        self.assertNotEqual(result.returncode, 0, output)
        self.assertIn(f"next checkpoint must be {self.next}", output)

        after = {relative: (self.repo / relative).read_bytes() for relative in MIRROR_PATHS}
        self.assertEqual(before, after)

        checker = run(self.repo, sys.executable, "tools/check_docs.py")
        self.assertEqual(checker.returncode, 0, checker.stdout + checker.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
